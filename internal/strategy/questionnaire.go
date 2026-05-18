package strategy

import (
	"fmt"
	"strconv"
)

// GetQuestions returns all questionnaire questions organized by group.
func GetQuestions() []Question {
	return []Question{
		// Group: Strategy
		{
			ID:          "strategy_type",
			Group:       "Strategy",
			Text:        "What migration strategy do you want to pursue?",
			Description: "The overall approach for migrating your COBOL codebase.",
			Type:        QuestionTypeSingleSelect,
			Required:    true,
			Options: []Option{
				{Value: "refactor", Label: "Refactor", Description: "Restructure code while keeping COBOL semantics"},
				{Value: "rewrite", Label: "Rewrite", Description: "Complete rewrite in a modern language"},
				{Value: "replatform", Label: "Replatform", Description: "Move to a modern platform with minimal code changes"},
				{Value: "rehost", Label: "Re-host", Description: "Lift and shift to new infrastructure"},
				{Value: "retire", Label: "Retire", Description: "Decommission and replace with COTS/SaaS"},
				{Value: "retain", Label: "Retain", Description: "Keep as-is with improved observability"},
				{Value: "integrate", Label: "Integrate", Description: "Wrap with APIs and integrate with modern systems"},
				{Value: "hybrid", Label: "Hybrid", Description: "Combine multiple strategies per program/domain"},
			},
		},
		{
			ID:          "hybrid_strategies",
			Group:       "Strategy",
			Text:        "Which strategies do you want to combine?",
			Description: "Select all strategies that apply to different parts of the codebase.",
			Type:        QuestionTypeMultiSelect,
			DependsOn:   &Dependency{QuestionID: "strategy_type", Values: []string{"hybrid"}},
			Options: []Option{
				{Value: "refactor", Label: "Refactor"},
				{Value: "rewrite", Label: "Rewrite"},
				{Value: "replatform", Label: "Replatform"},
				{Value: "rehost", Label: "Re-host"},
				{Value: "retire", Label: "Retire"},
				{Value: "retain", Label: "Retain"},
				{Value: "integrate", Label: "Integrate"},
			},
		},

		// Group: Current Stack
		{
			ID:          "current_database",
			Group:       "Current Stack",
			Text:        "What databases does the COBOL system use?",
			Description: "Select all database technologies currently in use.",
			Type:        QuestionTypeMultiSelect,
			Options: []Option{
				{Value: "db2", Label: "DB2"},
				{Value: "ims_db", Label: "IMS DB"},
				{Value: "vsam", Label: "VSAM"},
				{Value: "oracle", Label: "Oracle"},
				{Value: "sql_server", Label: "SQL Server"},
				{Value: "postgresql", Label: "PostgreSQL"},
				{Value: "other", Label: "Other"},
			},
		},
		{
			ID:        "current_database_other",
			Group:     "Current Stack",
			Text:      "Specify the other database(s)",
			Type:      QuestionTypeText,
			DependsOn: &Dependency{QuestionID: "current_database", Values: []string{"other"}},
		},
		{
			ID:          "current_middleware",
			Group:       "Current Stack",
			Text:        "What middleware/transaction monitors are in use?",
			Description: "Select all middleware technologies currently in use.",
			Type:        QuestionTypeMultiSelect,
			Options: []Option{
				{Value: "cics", Label: "CICS"},
				{Value: "ims_tm", Label: "IMS TM"},
				{Value: "mq_series", Label: "MQ Series"},
				{Value: "tibco", Label: "TIBCO"},
				{Value: "none", Label: "None"},
				{Value: "other", Label: "Other"},
			},
		},
		{
			ID:        "current_middleware_other",
			Group:     "Current Stack",
			Text:      "Specify the other middleware",
			Type:      QuestionTypeText,
			DependsOn: &Dependency{QuestionID: "current_middleware", Values: []string{"other"}},
		},
		{
			ID:          "current_batch",
			Group:       "Current Stack",
			Text:        "What batch scheduling system is used?",
			Description: "The job scheduler for batch COBOL processing.",
			Type:        QuestionTypeSingleSelect,
			Options: []Option{
				{Value: "jcl_jes", Label: "JCL/JES"},
				{Value: "control_m", Label: "Control-M"},
				{Value: "autosys", Label: "Autosys"},
				{Value: "tws", Label: "TWS"},
				{Value: "other", Label: "Other"},
				{Value: "none", Label: "None"},
			},
		},
		{
			ID:          "current_monitoring",
			Group:       "Current Stack",
			Text:        "What monitoring/APM tools are in use?",
			Description: "Current application performance monitoring tools (e.g., Datadog, Splunk, Dynatrace).",
			Type:        QuestionTypeText,
		},

		// Group: Target Platform
		{
			ID:          "target_language",
			Group:       "Target Platform",
			Text:        "What is the target programming language?",
			Description: "The primary language for the modernized codebase.",
			Type:        QuestionTypeSingleSelect,
			Required:    true,
			Options: []Option{
				{Value: "java", Label: "Java"},
				{Value: "csharp", Label: "C#"},
				{Value: "python", Label: "Python"},
				{Value: "go", Label: "Go"},
				{Value: "typescript", Label: "TypeScript"},
				{Value: "kotlin", Label: "Kotlin"},
				{Value: "other", Label: "Other"},
			},
		},
		{
			ID:        "target_language_other",
			Group:     "Target Platform",
			Text:      "Specify the target language",
			Type:      QuestionTypeText,
			DependsOn: &Dependency{QuestionID: "target_language", Values: []string{"other"}},
		},
		{
			ID:          "target_framework",
			Group:       "Target Platform",
			Text:        "What framework or runtime preferences do you have?",
			Description: "E.g., Spring Boot, .NET 8, Django, Express, etc.",
			Type:        QuestionTypeText,
		},
		{
			ID:          "target_platform",
			Group:       "Target Platform",
			Text:        "What is the target deployment platform?",
			Description: "Where the modernized application will run.",
			Type:        QuestionTypeSingleSelect,
			Options: []Option{
				{Value: "cloud-aws", Label: "Cloud (AWS)"},
				{Value: "cloud-azure", Label: "Cloud (Azure)"},
				{Value: "cloud-gcp", Label: "Cloud (GCP)"},
				{Value: "on-premise", Label: "On-premise"},
				{Value: "hybrid", Label: "Hybrid"},
			},
		},
		{
			ID:          "target_database",
			Group:       "Target Platform",
			Text:        "What is the target database?",
			Description: "The primary database for the modernized system.",
			Type:        QuestionTypeSingleSelect,
			Options: []Option{
				{Value: "postgresql", Label: "PostgreSQL"},
				{Value: "mysql", Label: "MySQL"},
				{Value: "aurora", Label: "Aurora"},
				{Value: "sql_server", Label: "SQL Server"},
				{Value: "cosmosdb", Label: "CosmosDB"},
				{Value: "dynamodb", Label: "DynamoDB"},
				{Value: "cloud_sql", Label: "Cloud SQL"},
				{Value: "keep_existing", Label: "Keep existing"},
			},
		},
		{
			ID:          "target_messaging",
			Group:       "Target Platform",
			Text:        "What messaging/event system do you want to use?",
			Description: "For async communication between modernized services.",
			Type:        QuestionTypeSingleSelect,
			Options: []Option{
				{Value: "kafka", Label: "Kafka"},
				{Value: "rabbitmq", Label: "RabbitMQ"},
				{Value: "sqs_sns", Label: "SQS/SNS"},
				{Value: "azure_service_bus", Label: "Azure Service Bus"},
				{Value: "pubsub", Label: "Pub/Sub"},
				{Value: "none", Label: "None"},
			},
		},
		{
			ID:          "target_api_style",
			Group:       "Target Platform",
			Text:        "What API style do you prefer?",
			Description: "The primary API paradigm for the modernized system.",
			Type:        QuestionTypeSingleSelect,
			Options: []Option{
				{Value: "rest", Label: "REST"},
				{Value: "grpc", Label: "gRPC"},
				{Value: "graphql", Label: "GraphQL"},
				{Value: "event_driven", Label: "Event-driven"},
			},
		},
		{
			ID:          "target_containerization",
			Group:       "Target Platform",
			Text:        "What containerization/orchestration platform?",
			Description: "How the modernized services will be deployed.",
			Type:        QuestionTypeSingleSelect,
			Options: []Option{
				{Value: "kubernetes", Label: "Kubernetes"},
				{Value: "ecs_fargate", Label: "ECS/Fargate"},
				{Value: "cloud_run", Label: "Cloud Run"},
				{Value: "docker_compose", Label: "Docker Compose"},
				{Value: "vms", Label: "VMs"},
				{Value: "none", Label: "None"},
			},
		},
		{
			ID:          "target_cicd",
			Group:       "Target Platform",
			Text:        "What CI/CD pipeline do you use or prefer?",
			Description: "E.g., GitHub Actions, Jenkins, Azure DevOps, GitLab CI, etc.",
			Type:        QuestionTypeText,
		},

		// Group: Constraints
		{
			ID:          "timeline",
			Group:       "Constraints",
			Text:        "What is the expected migration timeline?",
			Description: "Overall project duration.",
			Type:        QuestionTypeSingleSelect,
			Required:    true,
			Options: []Option{
				{Value: "3-6m", Label: "3-6 months"},
				{Value: "6-12m", Label: "6-12 months"},
				{Value: "1-2y", Label: "1-2 years"},
				{Value: "2-5y", Label: "2-5 years"},
			},
		},
		{
			ID:          "team_size",
			Group:       "Constraints",
			Text:        "How many developers will work on the migration?",
			Description: "Total developer headcount.",
			Type:        QuestionTypeNumber,
		},
		{
			ID:          "team_skills",
			Group:       "Constraints",
			Text:        "What skills does the team have?",
			Description: "Current team capabilities.",
			Type:        QuestionTypeMultiSelect,
			Options: []Option{
				{Value: "cobol", Label: "COBOL"},
				{Value: "target_lang", Label: "Target language"},
				{Value: "both", Label: "Both COBOL and target"},
				{Value: "retraining", Label: "Retraining needed"},
			},
		},

		// Group: Requirements
		{
			ID:          "compliance",
			Group:       "Requirements",
			Text:        "What compliance requirements apply?",
			Description: "Regulatory frameworks the migration must satisfy.",
			Type:        QuestionTypeMultiSelect,
			Options: []Option{
				{Value: "sox", Label: "SOX"},
				{Value: "hipaa", Label: "HIPAA"},
				{Value: "pci-dss", Label: "PCI-DSS"},
				{Value: "gdpr", Label: "GDPR"},
				{Value: "fedramp", Label: "FedRAMP"},
				{Value: "none", Label: "None"},
			},
		},
		{
			ID:          "integrations",
			Group:       "Requirements",
			Text:        "What external systems must be preserved?",
			Description: "APIs, databases, message queues, or third-party services to maintain.",
			Type:        QuestionTypeText,
		},
		{
			ID:          "priority_criteria",
			Group:       "Requirements",
			Text:        "What are your priority criteria?",
			Description: "Factors that should drive migration sequencing decisions.",
			Type:        QuestionTypeMultiSelect,
			Options: []Option{
				{Value: "speed", Label: "Speed"},
				{Value: "cost", Label: "Cost"},
				{Value: "risk", Label: "Risk minimization"},
				{Value: "business_value", Label: "Business value"},
				{Value: "tech_debt", Label: "Tech debt reduction"},
			},
		},
		{
			ID:          "phasing",
			Group:       "Requirements",
			Text:        "What phasing approach do you prefer?",
			Description: "How to roll out the migration over time.",
			Type:        QuestionTypeSingleSelect,
			Options: []Option{
				{Value: "big_bang", Label: "Big bang"},
				{Value: "incremental", Label: "Incremental (strangler fig)"},
				{Value: "domain_by_domain", Label: "Domain-by-domain"},
				{Value: "risk_based", Label: "Risk-based"},
			},
		},

		// Group: Context
		{
			ID:          "criticality",
			Group:       "Context",
			Text:        "What is the criticality level of this system?",
			Description: "How critical is this system to business operations.",
			Type:        QuestionTypeSingleSelect,
			Options: []Option{
				{Value: "mission_critical", Label: "Mission critical"},
				{Value: "business_critical", Label: "Business critical"},
				{Value: "non_critical", Label: "Non-critical"},
			},
		},
		{
			ID:          "downtime_tolerance",
			Group:       "Context",
			Text:        "What is the acceptable downtime during migration?",
			Description: "How much downtime can the business tolerate during cutover.",
			Type:        QuestionTypeSingleSelect,
			Options: []Option{
				{Value: "zero", Label: "Zero downtime"},
				{Value: "maintenance_windows", Label: "Maintenance windows"},
				{Value: "extended_cutover", Label: "Extended cutover OK"},
			},
		},
		{
			ID:          "additional_notes",
			Group:       "Context",
			Text:        "Any additional context or constraints?",
			Description: "Free-form notes about the migration project.",
			Type:        QuestionTypeText,
		},
	}
}

// ShouldShow evaluates whether a question should be displayed based on current answers.
func ShouldShow(q Question, answers map[string][]string) bool {
	if q.DependsOn == nil {
		return true
	}
	parentAnswers, ok := answers[q.DependsOn.QuestionID]
	if !ok {
		return false
	}
	for _, pa := range parentAnswers {
		for _, dv := range q.DependsOn.Values {
			if pa == dv {
				return true
			}
		}
	}
	return false
}

// ValidateAnswers validates the questionnaire answers and builds a StrategyContext.
func ValidateAnswers(answers map[string][]string) (*StrategyContext, error) {
	questions := GetQuestions()

	// Check required fields
	for _, q := range questions {
		if !q.Required {
			continue
		}
		if !ShouldShow(q, answers) {
			continue
		}
		vals, ok := answers[q.ID]
		if !ok || len(vals) == 0 || (len(vals) == 1 && vals[0] == "") {
			return nil, fmt.Errorf("required field %q (%s) is missing", q.ID, q.Text)
		}
	}

	ctx := &StrategyContext{}

	// Strategy
	ctx.Strategy = firstVal(answers, "strategy_type")
	ctx.HybridStrategies = answers["hybrid_strategies"]

	// Current Stack
	ctx.CurrentDatabase = answers["current_database"]
	ctx.CurrentDatabaseOther = firstVal(answers, "current_database_other")
	ctx.CurrentMiddleware = answers["current_middleware"]
	ctx.CurrentMiddlewareOther = firstVal(answers, "current_middleware_other")
	ctx.CurrentBatch = firstVal(answers, "current_batch")
	ctx.CurrentMonitoring = firstVal(answers, "current_monitoring")

	// Target Platform
	ctx.TargetLang = firstVal(answers, "target_language")
	ctx.TargetLangOther = firstVal(answers, "target_language_other")
	if ctx.TargetLang == "other" && ctx.TargetLangOther != "" {
		ctx.TargetLang = ctx.TargetLangOther
	}
	ctx.TargetFramework = firstVal(answers, "target_framework")
	ctx.TargetPlatform = firstVal(answers, "target_platform")
	ctx.TargetDatabase = firstVal(answers, "target_database")
	ctx.TargetMessaging = firstVal(answers, "target_messaging")
	ctx.TargetAPIStyle = firstVal(answers, "target_api_style")
	ctx.TargetContainerization = firstVal(answers, "target_containerization")
	ctx.TargetCICD = firstVal(answers, "target_cicd")

	// Constraints
	ctx.Timeline = firstVal(answers, "timeline")
	if ts := firstVal(answers, "team_size"); ts != "" {
		n, err := strconv.Atoi(ts)
		if err != nil {
			return nil, fmt.Errorf("team_size must be a number: %w", err)
		}
		ctx.TeamSize = n
	}
	ctx.TeamSkills = answers["team_skills"]

	// Requirements
	ctx.Compliance = answers["compliance"]
	ctx.Integrations = firstVal(answers, "integrations")
	ctx.PriorityCriteria = answers["priority_criteria"]
	ctx.PhasingApproach = firstVal(answers, "phasing")

	// Context
	ctx.Criticality = firstVal(answers, "criticality")
	ctx.DowntimeTolerance = firstVal(answers, "downtime_tolerance")
	ctx.AdditionalNotes = firstVal(answers, "additional_notes")

	return ctx, nil
}

func firstVal(answers map[string][]string, key string) string {
	vals, ok := answers[key]
	if !ok || len(vals) == 0 {
		return ""
	}
	return vals[0]
}
