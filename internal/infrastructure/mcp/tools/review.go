// Package tools provides MCP tool implementations.
package tools

import (
	"context"
	"fmt"
	"time"

	appReview "github.com/anthropics/claude-flow-go/internal/application/review"
	domainReview "github.com/anthropics/claude-flow-go/internal/domain/review"
	infraReview "github.com/anthropics/claude-flow-go/internal/infrastructure/review"
	"github.com/anthropics/claude-flow-go/internal/shared"
)

// ReviewTools provides MCP tools for adversarial review operations.
type ReviewTools struct {
	orchestrator    *appReview.ReviewOrchestrator
	swarmManager    *appReview.AdversarialSwarm
}

// NewReviewTools creates a new ReviewTools instance.
func NewReviewTools(
	orchestrator *appReview.ReviewOrchestrator,
	swarmManager *appReview.AdversarialSwarm,
) *ReviewTools {
	return &ReviewTools{
		orchestrator: orchestrator,
		swarmManager: swarmManager,
	}
}

// NewReviewToolsDefault creates ReviewTools with default components.
func NewReviewToolsDefault() *ReviewTools {
	orchestrator := appReview.NewReviewOrchestratorDefault()
	swarmManager := appReview.NewAdversarialSwarm()

	return &ReviewTools{
		orchestrator: orchestrator,
		swarmManager: swarmManager,
	}
}

// GetTools returns available review tools.
func (t *ReviewTools) GetTools() []shared.MCPTool {
	reviewTypeEnums := make([]string, 0)
	for _, rt := range domainReview.AllReviewTypes() {
		reviewTypeEnums = append(reviewTypeEnums, string(rt))
	}

	return []shared.MCPTool{
		{
			Name:        "review_request",
			Description: "Request an adversarial review for a work item",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workId": map[string]interface{}{
						"type":        "string",
						"description": "Unique identifier for the work to be reviewed",
					},
					"workType": map[string]interface{}{
						"type":        "string",
						"description": "Type of work (e.g., code, design, document)",
					},
					"authorId": map[string]interface{}{
						"type":        "string",
						"description": "ID of the work author",
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Title of the work",
					},
					"reviewType": map[string]interface{}{
						"type":        "string",
						"enum":        reviewTypeEnums,
						"description": "Type of review: security, quality, performance, architecture, compliance, full",
					},
					"urgency": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"low", "normal", "high", "critical"},
						"description": "Urgency level of the review",
					},
					"requiredReviewers": map[string]interface{}{
						"type":        "number",
						"description": "Number of reviewers required (default: 1)",
					},
					"specificReviewers": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
						"description": "Specific reviewer IDs to assign",
					},
					"useSwarm": map[string]interface{}{
						"type":        "boolean",
						"description": "Use multi-agent adversarial swarm for review",
					},
				},
				"required": []string{"workId", "workType", "authorId", "title", "reviewType"},
			},
		},
		{
			Name:        "review_verdict",
			Description: "Submit a review verdict",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"sessionId": map[string]interface{}{
						"type":        "string",
						"description": "Review session ID",
					},
					"reviewerId": map[string]interface{}{
						"type":        "string",
						"description": "Reviewer agent ID",
					},
					"reviewerType": map[string]interface{}{
						"type":        "string",
						"description": "Type of reviewer agent",
					},
					"decision": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"approve", "reject", "revise", "escalate"},
						"description": "Review decision",
					},
					"confidence": map[string]interface{}{
						"type":        "number",
						"description": "Confidence level (0-1)",
					},
					"issues": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"type":           map[string]interface{}{"type": "string"},
								"severity":       map[string]interface{}{"type": "string", "enum": []string{"low", "medium", "high", "critical"}},
								"title":          map[string]interface{}{"type": "string"},
								"description":    map[string]interface{}{"type": "string"},
								"recommendation": map[string]interface{}{"type": "string"},
								"isBlocker":      map[string]interface{}{"type": "boolean"},
							},
						},
						"description": "Issues found during review",
					},
					"recommendations": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
						"description": "Recommendations for improvement",
					},
					"summary": map[string]interface{}{
						"type":        "string",
						"description": "Brief summary of the review",
					},
				},
				"required": []string{"sessionId", "reviewerId", "decision"},
			},
		},
		{
			Name:        "review_challenge",
			Description: "Challenge a review decision",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"sessionId": map[string]interface{}{
						"type":        "string",
						"description": "Review session ID to challenge",
					},
					"challengerId": map[string]interface{}{
						"type":        "string",
						"description": "ID of the challenger",
					},
					"reason": map[string]interface{}{
						"type":        "string",
						"description": "Reason for the challenge",
					},
					"evidence": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
						"description": "Evidence supporting the challenge",
					},
				},
				"required": []string{"sessionId", "challengerId", "reason"},
			},
		},
		{
			Name:        "review_status",
			Description: "Get the status of a review session or workflow",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"sessionId": map[string]interface{}{
						"type":        "string",
						"description": "Review session ID",
					},
					"workflowId": map[string]interface{}{
						"type":        "string",
						"description": "Review workflow ID",
					},
					"swarmId": map[string]interface{}{
						"type":        "string",
						"description": "Review swarm ID",
					},
				},
			},
		},
		{
			Name:        "review_divergent",
			Description: "Detect divergent agents in consensus",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"votes": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"agentId":    map[string]interface{}{"type": "string"},
								"decision":   map[string]interface{}{"type": "boolean"},
								"confidence": map[string]interface{}{"type": "number"},
							},
						},
						"description": "Votes to analyze for divergence",
					},
					"threshold": map[string]interface{}{
						"type":        "number",
						"description": "Divergence threshold (0-1, default: 0.3)",
					},
					"getChronicDisagreers": map[string]interface{}{
						"type":        "boolean",
						"description": "Also return agents who chronically disagree",
					},
				},
				"required": []string{"votes"},
			},
		},
	}
}

// Execute executes a review tool.
func (t *ReviewTools) Execute(ctx context.Context, toolName string, params map[string]interface{}) (*shared.MCPToolResult, error) {
	switch toolName {
	case "review_request":
		return t.handleRequest(ctx, params)
	case "review_verdict":
		return t.handleVerdict(ctx, params)
	case "review_challenge":
		return t.handleChallenge(ctx, params)
	case "review_status":
		return t.handleStatus(ctx, params)
	case "review_divergent":
		return t.handleDivergent(ctx, params)
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

func (t *ReviewTools) handleRequest(ctx context.Context, params map[string]interface{}) (*shared.MCPToolResult, error) {
	workID, _ := params["workId"].(string)
	workType, _ := params["workType"].(string)
	authorID, _ := params["authorId"].(string)
	title, _ := params["title"].(string)
	reviewTypeStr, _ := params["reviewType"].(string)

	if workID == "" || workType == "" || authorID == "" || title == "" || reviewTypeStr == "" {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: workId, workType, authorId, title, and reviewType are required",
		}, nil
	}

	work := domainReview.Work{
		WorkID:    workID,
		Type:      workType,
		AuthorID:  authorID,
		Title:     title,
		CreatedAt: time.Now(),
	}

	opts := domainReview.ReviewOptions{
		ReviewType:        domainReview.ReviewType(reviewTypeStr),
		RequiredReviewers: 1,
	}

	if urgency, ok := params["urgency"].(string); ok {
		opts.Urgency = urgency
	}

	if requiredReviewers, ok := params["requiredReviewers"].(float64); ok {
		opts.RequiredReviewers = int(requiredReviewers)
	}

	if specificReviewers, ok := params["specificReviewers"].([]interface{}); ok {
		opts.SpecificReviewers = make([]string, len(specificReviewers))
		for i, r := range specificReviewers {
			if s, ok := r.(string); ok {
				opts.SpecificReviewers[i] = s
			}
		}
	}

	useSwarm, _ := params["useSwarm"].(bool)

	if useSwarm {
		// Use swarm-based review
		swarm, err := t.swarmManager.SpawnReviewSwarm(ctx, work, appReview.DefaultReviewSwarmConfig())
		if err != nil {
			return &shared.MCPToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}

		return &shared.MCPToolResult{
			Success: true,
			Data: map[string]interface{}{
				"swarmId":     swarm.SwarmID,
				"workId":      swarm.WorkID,
				"status":      string(swarm.Status),
				"memberCount": len(swarm.Members),
				"members":     swarmMembersToMap(swarm.Members),
			},
		}, nil
	}

	// Use standard review
	session, err := t.orchestrator.RequestReview(ctx, work, opts)
	if err != nil {
		return &shared.MCPToolResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &shared.MCPToolResult{
		Success: true,
		Data: map[string]interface{}{
			"sessionId":     session.SessionID,
			"workId":        session.WorkID,
			"reviewType":    string(session.ReviewType),
			"status":        string(session.Status),
			"reviewerCount": len(session.Reviewers),
			"reviewers":     reviewersToMap(session.Reviewers),
		},
	}, nil
}

func (t *ReviewTools) handleVerdict(ctx context.Context, params map[string]interface{}) (*shared.MCPToolResult, error) {
	sessionID, _ := params["sessionId"].(string)
	reviewerID, _ := params["reviewerId"].(string)
	decisionStr, _ := params["decision"].(string)

	if sessionID == "" || reviewerID == "" || decisionStr == "" {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: sessionId, reviewerId, and decision are required",
		}, nil
	}

	verdict := domainReview.ReviewVerdict{
		SessionID:    sessionID,
		ReviewerID:   reviewerID,
		Decision:     domainReview.ReviewDecision(decisionStr),
		StartedAt:    time.Now().Add(-1 * time.Minute), // Approximate
		Confidence:   0.8,                              // Default
		Issues:       make([]domainReview.ReviewIssue, 0),
		Recommendations: make([]string, 0),
	}

	if reviewerType, ok := params["reviewerType"].(string); ok {
		verdict.ReviewerType = reviewerType
	}

	if confidence, ok := params["confidence"].(float64); ok {
		verdict.Confidence = confidence
	}

	if summary, ok := params["summary"].(string); ok {
		verdict.Summary = summary
	}

	if issuesRaw, ok := params["issues"].([]interface{}); ok {
		for _, issueRaw := range issuesRaw {
			if issueMap, ok := issueRaw.(map[string]interface{}); ok {
				issue := domainReview.ReviewIssue{}
				if t, ok := issueMap["type"].(string); ok {
					issue.Type = t
				}
				if s, ok := issueMap["severity"].(string); ok {
					issue.Severity = domainReview.Severity(s)
				}
				if t, ok := issueMap["title"].(string); ok {
					issue.Title = t
				}
				if d, ok := issueMap["description"].(string); ok {
					issue.Description = d
				}
				if r, ok := issueMap["recommendation"].(string); ok {
					issue.Recommendation = r
				}
				if b, ok := issueMap["isBlocker"].(bool); ok {
					issue.IsBlocker = b
				}
				verdict.Issues = append(verdict.Issues, issue)
			}
		}
	}

	if recsRaw, ok := params["recommendations"].([]interface{}); ok {
		for _, rec := range recsRaw {
			if s, ok := rec.(string); ok {
				verdict.Recommendations = append(verdict.Recommendations, s)
			}
		}
	}

	err := t.orchestrator.SubmitVerdict(ctx, sessionID, verdict)
	if err != nil {
		return &shared.MCPToolResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &shared.MCPToolResult{
		Success: true,
		Data: map[string]interface{}{
			"submitted":  true,
			"sessionId":  sessionID,
			"reviewerId": reviewerID,
			"decision":   decisionStr,
			"issueCount": len(verdict.Issues),
		},
	}, nil
}

func (t *ReviewTools) handleChallenge(ctx context.Context, params map[string]interface{}) (*shared.MCPToolResult, error) {
	sessionID, _ := params["sessionId"].(string)
	challengerID, _ := params["challengerId"].(string)
	reason, _ := params["reason"].(string)

	if sessionID == "" || challengerID == "" || reason == "" {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: sessionId, challengerId, and reason are required",
		}, nil
	}

	var evidence []string
	if evidenceRaw, ok := params["evidence"].([]interface{}); ok {
		for _, e := range evidenceRaw {
			if s, ok := e.(string); ok {
				evidence = append(evidence, s)
			}
		}
	}

	challenge, err := t.orchestrator.ChallengeDecision(sessionID, challengerID, reason, evidence)
	if err != nil {
		return &shared.MCPToolResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &shared.MCPToolResult{
		Success: true,
		Data: map[string]interface{}{
			"challengeId":   challenge.ChallengeID,
			"workId":        challenge.WorkID,
			"sessionId":     challenge.SessionID,
			"status":        string(challenge.Status),
			"expiresAt":     challenge.ExpiresAt.String(),
			"contestWindow": challenge.ContestWindow.String(),
		},
	}, nil
}

func (t *ReviewTools) handleStatus(ctx context.Context, params map[string]interface{}) (*shared.MCPToolResult, error) {
	sessionID, hasSession := params["sessionId"].(string)
	workflowID, hasWorkflow := params["workflowId"].(string)
	swarmID, hasSwarm := params["swarmId"].(string)

	if hasSwarm && swarmID != "" {
		swarm, ok := t.swarmManager.GetSwarm(swarmID)
		if !ok {
			return &shared.MCPToolResult{
				Success: false,
				Error:   fmt.Sprintf("swarm not found: %s", swarmID),
			}, nil
		}

		data := map[string]interface{}{
			"swarmId":     swarm.SwarmID,
			"workId":      swarm.WorkID,
			"status":      string(swarm.Status),
			"memberCount": len(swarm.Members),
			"members":     swarmMembersToMap(swarm.Members),
			"createdAt":   swarm.CreatedAt.String(),
		}

		if swarm.FinalVerdict != nil {
			data["finalVerdict"] = map[string]interface{}{
				"decision":       string(swarm.FinalVerdict.Decision),
				"consensusLevel": swarm.FinalVerdict.ConsensusLevel,
				"blockerCount":   len(swarm.FinalVerdict.Blockers),
			}
		}

		if swarm.CompletedAt != nil {
			data["completedAt"] = swarm.CompletedAt.String()
		}

		return &shared.MCPToolResult{
			Success: true,
			Data:    data,
		}, nil
	}

	if hasWorkflow && workflowID != "" {
		workflow, err := t.orchestrator.GetWorkflowStatus(workflowID)
		if err != nil {
			return &shared.MCPToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}

		return &shared.MCPToolResult{
			Success: true,
			Data: map[string]interface{}{
				"workflowId":        workflow.WorkflowID,
				"workId":            workflow.WorkID,
				"status":            string(workflow.Status),
				"currentStage":      workflow.CurrentStage,
				"totalStages":       len(workflow.Stages),
				"requiredApprovals": workflow.RequiredApprovals,
				"currentApprovals":  workflow.CurrentApprovals,
				"blockerCount":      len(workflow.Blockers),
				"challengeCount":    len(workflow.Challenges),
				"finalDecision":     workflow.FinalDecision,
			},
		}, nil
	}

	if hasSession && sessionID != "" {
		session, err := t.orchestrator.GetReviewStatus(sessionID)
		if err != nil {
			return &shared.MCPToolResult{
				Success: false,
				Error:   err.Error(),
			}, nil
		}

		return &shared.MCPToolResult{
			Success: true,
			Data: map[string]interface{}{
				"sessionId":     session.SessionID,
				"workId":        session.WorkID,
				"reviewType":    string(session.ReviewType),
				"status":        string(session.Status),
				"reviewerCount": len(session.Reviewers),
				"verdictCount":  len(session.Verdicts),
				"createdAt":     session.CreatedAt.String(),
			},
		}, nil
	}

	// Return overall stats
	stats := t.orchestrator.GetStats()
	swarmStats := t.swarmManager.GetSwarmStats()

	return &shared.MCPToolResult{
		Success: true,
		Data: map[string]interface{}{
			"stats": map[string]interface{}{
				"totalReviews":         stats.TotalReviews,
				"activeSessions":       stats.ActiveSessions,
				"completedReviews":     stats.CompletedReviews,
				"approvalRate":         stats.ApprovalRate,
				"avgReviewDurationMs":  stats.AvgReviewDurationMs,
				"totalChallenges":      stats.TotalChallenges,
				"challengeSuccessRate": stats.ChallengeSuccessRate,
			},
			"swarmStats": swarmStats,
		},
	}, nil
}

func (t *ReviewTools) handleDivergent(ctx context.Context, params map[string]interface{}) (*shared.MCPToolResult, error) {
	votesRaw, ok := params["votes"].([]interface{})
	if !ok || len(votesRaw) == 0 {
		return &shared.MCPToolResult{
			Success: false,
			Error:   "validation: votes array is required",
		}, nil
	}

	votes := make([]infraReview.Vote, 0, len(votesRaw))
	for _, voteRaw := range votesRaw {
		if voteMap, ok := voteRaw.(map[string]interface{}); ok {
			vote := infraReview.Vote{
				Timestamp: time.Now(),
			}
			if agentID, ok := voteMap["agentId"].(string); ok {
				vote.AgentID = agentID
			}
			if decision, ok := voteMap["decision"].(bool); ok {
				vote.Decision = decision
			}
			if confidence, ok := voteMap["confidence"].(float64); ok {
				vote.Confidence = confidence
			}
			votes = append(votes, vote)
		}
	}

	threshold := 0.3
	if thresholdVal, ok := params["threshold"].(float64); ok {
		threshold = thresholdVal
	}

	detector := t.orchestrator.GetDivergentDetector()
	divergent := detector.DetectDivergent(votes, threshold)

	result := make([]map[string]interface{}, len(divergent))
	for i, d := range divergent {
		result[i] = map[string]interface{}{
			"agentId":          d.AgentID,
			"distance":         d.Distance,
			"disagreementRate": d.DisagreementRate,
			"reason":           d.Reason,
		}
	}

	response := map[string]interface{}{
		"divergentAgents": result,
		"totalVotes":      len(votes),
		"threshold":       threshold,
	}

	if getChronicDisagreers, ok := params["getChronicDisagreers"].(bool); ok && getChronicDisagreers {
		chronic := detector.GetChronicDisagreers()
		chronicResult := make([]map[string]interface{}, len(chronic))
		for i, c := range chronic {
			chronicResult[i] = map[string]interface{}{
				"agentId":          c.AgentID,
				"disagreementRate": c.DisagreementRate,
				"reason":           c.Reason,
			}
		}
		response["chronicDisagreers"] = chronicResult
	}

	return &shared.MCPToolResult{
		Success: true,
		Data:    response,
	}, nil
}

// GetOrchestrator returns the review orchestrator.
func (t *ReviewTools) GetOrchestrator() *appReview.ReviewOrchestrator {
	return t.orchestrator
}

// GetSwarmManager returns the swarm manager.
func (t *ReviewTools) GetSwarmManager() *appReview.AdversarialSwarm {
	return t.swarmManager
}

// Helper functions

func reviewersToMap(reviewers []domainReview.ReviewerAssignment) []map[string]interface{} {
	result := make([]map[string]interface{}, len(reviewers))
	for i, r := range reviewers {
		result[i] = map[string]interface{}{
			"reviewerId":   r.ReviewerID,
			"reviewerType": r.ReviewerType,
			"role":         string(r.Role),
			"hasSubmitted": r.HasSubmitted,
		}
	}
	return result
}

func swarmMembersToMap(members []appReview.ReviewSwarmMember) []map[string]interface{} {
	result := make([]map[string]interface{}, len(members))
	for i, m := range members {
		result[i] = map[string]interface{}{
			"agentId":    m.AgentID,
			"agentType":  string(m.AgentType),
			"reviewType": string(m.ReviewType),
			"role":       string(m.Role),
			"status":     m.Status,
		}
	}
	return result
}
