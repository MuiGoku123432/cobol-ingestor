package targetstack

import (
	"encoding/json"
	"fmt"
	"strings"

	"cobol-ingestor/internal/graph"
)

// extractionJSON is the JSON schema returned by the LLM for per-file extraction.
type extractionJSON struct {
	Services      []tsServiceJSON      `json:"services"`
	Endpoints     []tsEndpointJSON     `json:"endpoints"`
	Rules         []tsRuleJSON         `json:"rules"`
	DataModels    []tsDataModelJSON    `json:"dataModels"`
	Integrations  []tsIntegrationJSON  `json:"integrations"`
	ErrorHandlers []tsErrorHandlerJSON `json:"errorHandlers"`
}

type tsServiceJSON struct {
	Name        string `json:"name"`
	ServiceType string `json:"serviceType"`
	Description string `json:"description"`
	BasePath    string `json:"basePath"`
	Language    string `json:"language"`
	Framework   string `json:"framework"`
}

type tsEndpointJSON struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Description string `json:"description"`
	ServiceName string `json:"serviceName"`
	Parameters  string `json:"parameters"`
}

type tsRuleJSON struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	ServiceName string  `json:"serviceName"`
	Confidence  float64 `json:"confidence"`
}

type tsDataModelJSON struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ServiceName string `json:"serviceName"`
	Fields      string `json:"fields"`
	TableName   string `json:"tableName"`
}

type tsIntegrationJSON struct {
	IntegrationType string `json:"integrationType"`
	Target          string `json:"target"`
	Description     string `json:"description"`
	ServiceName     string `json:"serviceName"`
}

type tsErrorHandlerJSON struct {
	Pattern     string `json:"pattern"`
	Description string `json:"description"`
	ServiceName string `json:"serviceName"`
}

// ParseExtractionResponse parses the LLM JSON output for a single source file.
func ParseExtractionResponse(jsonStr, sourceFile, repoURL string) (*ExtractionResult, error) {
	cleaned := stripMarkdownFences(jsonStr)

	var raw extractionJSON
	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		return nil, fmt.Errorf("parsing target stack JSON: %w\nraw: %.500s", err, cleaned)
	}

	result := &ExtractionResult{SourceFile: sourceFile}

	for _, s := range raw.Services {
		if s.Name == "" {
			continue
		}
		result.Services = append(result.Services, graph.TargetService{
			ID:          repoURL + "::" + s.Name,
			Name:        s.Name,
			ServiceType: normalizeServiceType(s.ServiceType),
			Description: s.Description,
			RepoURL:     repoURL,
			BasePath:    s.BasePath,
			Language:    s.Language,
			Framework:   s.Framework,
		})
	}

	for _, e := range raw.Endpoints {
		if e.Path == "" {
			continue
		}
		result.Endpoints = append(result.Endpoints, graph.TargetEndpoint{
			ID:          repoURL + "::" + e.ServiceName + "::" + e.Method + "::" + e.Path,
			Method:      strings.ToUpper(e.Method),
			Path:        e.Path,
			Description: e.Description,
			ServiceName: e.ServiceName,
			Parameters:  e.Parameters,
		})
	}

	for _, r := range raw.Rules {
		if r.Name == "" || r.Confidence < 0.5 {
			continue
		}
		result.Rules = append(result.Rules, graph.TargetBusinessRule{
			ID:          repoURL + "::" + r.ServiceName + "::" + r.Name,
			Name:        r.Name,
			Description: r.Description,
			Category:    normalizeRuleCategory(r.Category),
			ServiceName: r.ServiceName,
			SourceFile:  sourceFile,
			Confidence:  r.Confidence,
		})
	}

	for _, m := range raw.DataModels {
		if m.Name == "" {
			continue
		}
		result.DataModels = append(result.DataModels, graph.TargetDataModel{
			ID:          repoURL + "::" + m.ServiceName + "::" + m.Name,
			Name:        m.Name,
			Description: m.Description,
			ServiceName: m.ServiceName,
			SourceFile:  sourceFile,
			Fields:      m.Fields,
			TableName:   m.TableName,
		})
	}

	for _, i := range raw.Integrations {
		if i.Target == "" {
			continue
		}
		result.Integrations = append(result.Integrations, graph.TargetIntegration{
			ID:              repoURL + "::" + i.ServiceName + "::" + i.IntegrationType + "::" + i.Target,
			IntegrationType: normalizeIntegrationType(i.IntegrationType),
			Target:          i.Target,
			Description:     i.Description,
			ServiceName:     i.ServiceName,
		})
	}

	for _, eh := range raw.ErrorHandlers {
		if eh.Pattern == "" {
			continue
		}
		result.ErrorHandlers = append(result.ErrorHandlers, graph.TargetErrorHandler{
			ID:          repoURL + "::" + eh.ServiceName + "::" + eh.Pattern,
			Pattern:     normalizeErrorPattern(eh.Pattern),
			Description: eh.Description,
			ServiceName: eh.ServiceName,
			SourceFile:  sourceFile,
		})
	}

	return result, nil
}

func normalizeServiceType(s string) string {
	switch strings.ToUpper(s) {
	case "REST_API", "REST", "HTTP":
		return "REST_API"
	case "GRPC":
		return "GRPC"
	case "MESSAGE_CONSUMER", "CONSUMER", "SUBSCRIBER":
		return "MESSAGE_CONSUMER"
	case "BATCH_JOB", "BATCH", "JOB":
		return "BATCH_JOB"
	case "LIBRARY", "LIB", "UTIL", "UTILITY":
		return "LIBRARY"
	default:
		return "REST_API"
	}
}

func normalizeRuleCategory(s string) string {
	switch strings.ToUpper(s) {
	case "VALIDATION", "VALIDATE":
		return "VALIDATION"
	case "CALCULATION", "CALC", "COMPUTE":
		return "CALCULATION"
	case "AUTHORIZATION", "AUTH", "AUTHZ":
		return "AUTHORIZATION"
	case "WORKFLOW", "FLOW", "ORCHESTRATION":
		return "WORKFLOW"
	case "TRANSFORMATION", "TRANSFORM", "MAPPING":
		return "TRANSFORMATION"
	default:
		return "VALIDATION"
	}
}

func normalizeIntegrationType(s string) string {
	switch strings.ToUpper(s) {
	case "DATABASE", "DB":
		return "DATABASE"
	case "REST_CLIENT", "REST", "HTTP_CLIENT":
		return "REST_CLIENT"
	case "MESSAGE_QUEUE", "MQ", "QUEUE", "KAFKA", "RABBITMQ", "SQS":
		return "MESSAGE_QUEUE"
	case "FILE_IO", "FILE", "FILESYSTEM":
		return "FILE_IO"
	case "CACHE", "REDIS", "MEMCACHE":
		return "CACHE"
	default:
		return "EXTERNAL_API"
	}
}

func normalizeErrorPattern(s string) string {
	switch strings.ToUpper(s) {
	case "TRY_CATCH", "TRY/CATCH", "EXCEPTION":
		return "TRY_CATCH"
	case "ERROR_MIDDLEWARE", "MIDDLEWARE", "HANDLER":
		return "ERROR_MIDDLEWARE"
	case "CIRCUIT_BREAKER", "CIRCUIT":
		return "CIRCUIT_BREAKER"
	case "RETRY":
		return "RETRY"
	case "FALLBACK", "DEFAULT":
		return "FALLBACK"
	default:
		return "TRY_CATCH"
	}
}

// stripMarkdownFences removes ```json ... ``` wrappers from LLM responses.
func stripMarkdownFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		// find first newline after the fence
		idx := strings.Index(s, "\n")
		if idx != -1 {
			s = s[idx+1:]
		}
	}
	if strings.HasSuffix(s, "```") {
		s = s[:strings.LastIndex(s, "```")]
	}
	return strings.TrimSpace(s)
}
