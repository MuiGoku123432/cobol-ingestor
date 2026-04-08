package targetstack

import (
	"context"
	"encoding/json"
	"fmt"

	"cobol-ingestor/internal/graph"
	"cobol-ingestor/internal/llm"
	"cobol-ingestor/internal/neo4j"

	"go.uber.org/zap"
)

const requirementsSystemPrompt = `You are a business analyst generating formal business requirements for a COBOL modernization project.

You have access to tools that let you query:
- cobol_* tools: the COBOL mainframe knowledge graph
- target_* tools: the modern target stack graph, including business gaps and coverage summaries

## Your Task

1. Use target_list_business_gaps to retrieve all identified gaps (start with CRITICAL, then HIGH severity)
2. Use target_get_gap_coverage_summary to understand overall coverage
3. For each significant gap, investigate the COBOL source using cobol_* tools to understand the full business context
4. Generate comprehensive business requirements that would close the gaps

## Requirement Quality Standards

Each requirement must:
- Have a clear, actionable title starting with a verb (Implement, Add, Migrate, Replace)
- Describe the business context (what does the COBOL do, why does the target stack need it)
- Include 3-5 specific, testable acceptance criteria
- Have a realistic effort estimate based on gap complexity
- Reference the specific COBOL program/function being replaced

Group related gaps into single requirements where appropriate.

## Output

When you have gathered sufficient information, output your requirements as a JSON array in a <requirements> XML block.

Format:
<requirements>
[
  {
    "id": "req-001",
    "title": "Implement customer credit limit validation",
    "description": "...",
    "priority": "P0",
    "category": "BUSINESS_RULE",
    "acceptanceCriteria": "[\"...\", \"...\"]",
    "estimatedEffort": "M",
    "gapId": "gap-business_rule-..."
  }
]
</requirements>`

// GenerateRequirements runs an agentic loop that reads gaps from Neo4j
// and generates BusinessRequirement objects. Returns requirements to write to Neo4j.
func GenerateRequirements(ctx context.Context, bridge *GapBridge, provider llm.ChatProvider, model string, maxTokens, maxIter int, logger *zap.Logger) ([]graph.BusinessRequirement, error) {
	tools, err := bridge.ListAllTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing tools: %w", err)
	}

	messages := []llm.ChatMessage{
		{
			Role:    llm.RoleUser,
			Content: []llm.ContentBlock{llm.NewTextContent("Generate business requirements from the identified gaps. Start by retrieving the gap list and coverage summary, then investigate key gaps before writing requirements.")},
		},
	}

	for i := range maxIter {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		resp, err := provider.CompleteChat(ctx, llm.ChatRequest{
			Model:     model,
			System:    requirementsSystemPrompt,
			MaxTokens: maxTokens,
			Tools:     tools,
			Messages:  messages,
		})
		if err != nil {
			return nil, fmt.Errorf("requirements iteration %d: %w", i+1, err)
		}

		if resp.StopReason != "tool_use" {
			text := resp.TextContent()
			reqs := extractRequirementsFromCoordinator(text)
			logger.Info("requirements generation complete", zap.Int("requirements", len(reqs)))
			return reqs, nil
		}

		messages = append(messages, llm.ChatMessage{
			Role:    llm.RoleAssistant,
			Content: resp.Content,
		})

		var toolResults []llm.ContentBlock
		for _, block := range resp.ToolUseBlocks() {
			var args map[string]any
			_ = json.Unmarshal(block.Input, &args)
			result, callErr := bridge.CallTool(ctx, block.Name, args)
			if callErr != nil {
				toolResults = append(toolResults, llm.NewToolResultContent(block.ID, fmt.Sprintf("Error: %v", callErr), true))
			} else {
				toolResults = append(toolResults, llm.NewToolResultContent(block.ID, result, false))
			}
		}
		messages = append(messages, llm.ChatMessage{Role: llm.RoleUser, Content: toolResults})
	}

	return nil, fmt.Errorf("requirements agent max iterations reached")
}

// RepoFromCloneResult builds a graph.TargetRepo from a CloneResult and scan/analysis data.
func RepoFromCloneResult(cr *CloneResult, scan *ScanResult, analysis *graph.TargetStackResult, cloneBaseDir string) graph.TargetRepo {
	lang := detectPrimaryLanguage(scan)
	framework := detectPrimaryFramework(analysis)

	return graph.TargetRepo{
		ID:         cr.Config.URL,
		Name:       RepoName(cr.Config.URL),
		URL:        cr.Config.URL,
		Provider:   DetectProvider(cr.Config.URL),
		Branch:     cr.Config.Branch,
		LastCommit: cr.HeadSHA,
		LocalPath:  cr.LocalPath,
		Language:   lang,
		Framework:  framework,
	}
}

func detectPrimaryLanguage(scan *ScanResult) string {
	if scan == nil {
		return ""
	}
	best := ""
	bestCount := 0
	for lang, count := range scan.ByLang {
		if count > bestCount && lang != "XML" && lang != "YAML" && lang != "JSON" {
			bestCount = count
			best = lang
		}
	}
	return best
}

func detectPrimaryFramework(analysis *graph.TargetStackResult) string {
	if analysis == nil {
		return ""
	}
	for _, s := range analysis.Services {
		if s.Framework != "" {
			return s.Framework
		}
	}
	return ""
}

// HeadSHAFromNeo4j retrieves the last-known HEAD SHA for a repo from Neo4j.
// Returns empty string if not found (repo not previously analyzed).
func HeadSHAFromNeo4j(ctx context.Context, client *neo4j.Client, repoURL string) string {
	repos, err := client.ListTargetRepos(ctx)
	if err != nil {
		return ""
	}
	for _, r := range repos {
		if r.URL == repoURL {
			return r.LastCommit
		}
	}
	return ""
}
