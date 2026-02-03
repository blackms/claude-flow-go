// Package neural provides neural learning application services.
package neural

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	domainNeural "github.com/anthropics/claude-flow-go/internal/domain/neural"
	infraNeural "github.com/anthropics/claude-flow-go/internal/infrastructure/neural"
)

// RLService orchestrates reinforcement learning algorithms.
type RLService struct {
	mu sync.RWMutex

	// Active algorithms
	algorithms map[string]RLAlgorithmWrapper

	// Curiosity module (optional)
	curiosityModule *infraNeural.CuriosityModule

	// Multi-agent system (optional)
	multiAgentRL *infraNeural.MultiAgentRL

	// Imitation learner (optional)
	imitationLearner *infraNeural.ImitationLearner

	// Checkpoint directory
	checkpointDir string

	// Statistics
	totalUpdates int64
	startTime    time.Time
}

// RLAlgorithmWrapper wraps different RL algorithm implementations.
type RLAlgorithmWrapper interface {
	AddExperience(exp domainNeural.RLExperience)
	Update() domainNeural.RLUpdateResult
	GetAction(state []float32, explore bool) int
	GetStats() domainNeural.RLStats
	Reset()
}

// NewRLService creates a new RL service.
func NewRLService(checkpointDir string) *RLService {
	return &RLService{
		algorithms:    make(map[string]RLAlgorithmWrapper),
		checkpointDir: checkpointDir,
		startTime:     time.Now(),
	}
}

// CreateAlgorithm creates a new RL algorithm instance.
func (s *RLService) CreateAlgorithm(name string, algorithm domainNeural.RLAlgorithm, config interface{}) (RLAlgorithmWrapper, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var alg RLAlgorithmWrapper

	switch algorithm {
	case domainNeural.AlgorithmPPO:
		cfg, ok := config.(domainNeural.PPOConfig)
		if !ok {
			cfg = domainNeural.DefaultPPOConfig()
		}
		alg = infraNeural.NewPPOAlgorithm(cfg)

	case domainNeural.AlgorithmDQN:
		cfg, ok := config.(domainNeural.DQNConfig)
		if !ok {
			cfg = domainNeural.DefaultDQNConfig()
		}
		alg = infraNeural.NewDQNAlgorithm(cfg)

	case domainNeural.AlgorithmA2C:
		cfg, ok := config.(domainNeural.A2CConfig)
		if !ok {
			cfg = domainNeural.DefaultA2CConfig()
		}
		alg = infraNeural.NewA2CAlgorithm(cfg)

	case domainNeural.AlgorithmQLearning:
		cfg, ok := config.(domainNeural.QLearningConfig)
		if !ok {
			cfg = domainNeural.DefaultQLearningConfig()
		}
		alg = infraNeural.NewQLearningAlgorithm(cfg)

	case domainNeural.AlgorithmSARSA:
		cfg, ok := config.(domainNeural.SARSAConfig)
		if !ok {
			cfg = domainNeural.DefaultSARSAConfig()
		}
		alg = infraNeural.NewSARSAAlgorithm(cfg)

	case domainNeural.AlgorithmCuriosity:
		cfg, ok := config.(domainNeural.CuriosityConfig)
		if !ok {
			cfg = domainNeural.DefaultCuriosityConfig()
		}
		s.curiosityModule = infraNeural.NewCuriosityModule(cfg)
		return nil, nil // Curiosity is a module, not standalone algorithm

	case domainNeural.AlgorithmImitation:
		cfg, ok := config.(domainNeural.ImitationConfig)
		if !ok {
			cfg = domainNeural.DefaultImitationConfig()
		}
		s.imitationLearner = infraNeural.NewImitationLearner(cfg)
		alg = s.imitationLearner

	case domainNeural.AlgorithmMultiAgent:
		cfg, ok := config.(domainNeural.MultiAgentConfig)
		if !ok {
			cfg = domainNeural.DefaultMultiAgentConfig()
		}
		s.multiAgentRL = infraNeural.NewMultiAgentRL(cfg)
		return nil, nil // Multi-agent is managed separately

	default:
		return nil, fmt.Errorf("unknown algorithm: %s", algorithm)
	}

	if alg != nil {
		s.algorithms[name] = alg
	}

	return alg, nil
}

// GetAlgorithm returns a registered algorithm.
func (s *RLService) GetAlgorithm(name string) (RLAlgorithmWrapper, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	alg, ok := s.algorithms[name]
	return alg, ok
}

// ListAlgorithms returns all registered algorithm names.
func (s *RLService) ListAlgorithms() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.algorithms))
	for name := range s.algorithms {
		names = append(names, name)
	}
	return names
}

// DeleteAlgorithm removes an algorithm.
func (s *RLService) DeleteAlgorithm(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.algorithms[name]; ok {
		delete(s.algorithms, name)
		return true
	}
	return false
}

// GetDefaultConfig returns the default configuration for an algorithm.
func (s *RLService) GetDefaultConfig(algorithm domainNeural.RLAlgorithm) interface{} {
	switch algorithm {
	case domainNeural.AlgorithmPPO:
		return domainNeural.DefaultPPOConfig()
	case domainNeural.AlgorithmDQN:
		return domainNeural.DefaultDQNConfig()
	case domainNeural.AlgorithmA2C:
		return domainNeural.DefaultA2CConfig()
	case domainNeural.AlgorithmQLearning:
		return domainNeural.DefaultQLearningConfig()
	case domainNeural.AlgorithmSARSA:
		return domainNeural.DefaultSARSAConfig()
	case domainNeural.AlgorithmCuriosity:
		return domainNeural.DefaultCuriosityConfig()
	case domainNeural.AlgorithmImitation:
		return domainNeural.DefaultImitationConfig()
	case domainNeural.AlgorithmMultiAgent:
		return domainNeural.DefaultMultiAgentConfig()
	default:
		return domainNeural.DefaultRLConfig()
	}
}

// SelectAlgorithm recommends an algorithm based on task type.
func (s *RLService) SelectAlgorithm(taskType string) domainNeural.RLAlgorithm {
	switch taskType {
	case "continuous-control", "robotics":
		return domainNeural.AlgorithmPPO
	case "discrete-actions", "games":
		return domainNeural.AlgorithmDQN
	case "real-time", "low-latency":
		return domainNeural.AlgorithmA2C
	case "simple", "tabular", "small-state":
		return domainNeural.AlgorithmQLearning
	case "safe", "conservative":
		return domainNeural.AlgorithmSARSA
	case "exploration", "sparse-reward":
		return domainNeural.AlgorithmCuriosity
	case "demonstration", "expert":
		return domainNeural.AlgorithmImitation
	case "multi-agent", "team", "swarm":
		return domainNeural.AlgorithmMultiAgent
	case "sequence", "trajectory":
		return domainNeural.AlgorithmDecisionTransformer
	default:
		return domainNeural.AlgorithmPPO // Default to PPO
	}
}

// AddExperience adds experience to a specific algorithm.
func (s *RLService) AddExperience(algorithmName string, exp domainNeural.RLExperience) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	alg, ok := s.algorithms[algorithmName]
	if !ok {
		return fmt.Errorf("algorithm not found: %s", algorithmName)
	}

	alg.AddExperience(exp)
	return nil
}

// AddExperienceWithCuriosity adds experience and computes curiosity bonus.
func (s *RLService) AddExperienceWithCuriosity(algorithmName string, exp domainNeural.RLExperience) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	alg, ok := s.algorithms[algorithmName]
	if !ok {
		return 0, fmt.Errorf("algorithm not found: %s", algorithmName)
	}

	var intrinsicReward float64
	if s.curiosityModule != nil {
		intrinsicReward = s.curiosityModule.ComputeIntrinsicReward(exp.State, exp.NextState, exp.Action)
		exp.Reward += intrinsicReward
	}

	alg.AddExperience(exp)
	return intrinsicReward, nil
}

// Update performs update on a specific algorithm.
func (s *RLService) Update(algorithmName string) (domainNeural.RLUpdateResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	alg, ok := s.algorithms[algorithmName]
	if !ok {
		return domainNeural.RLUpdateResult{}, fmt.Errorf("algorithm not found: %s", algorithmName)
	}

	result := alg.Update()
	s.totalUpdates++
	return result, nil
}

// UpdateAll performs update on all registered algorithms.
func (s *RLService) UpdateAll() map[string]domainNeural.RLUpdateResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	results := make(map[string]domainNeural.RLUpdateResult)
	for name, alg := range s.algorithms {
		results[name] = alg.Update()
		s.totalUpdates++
	}

	// Update curiosity module if enabled
	if s.curiosityModule != nil {
		// Note: curiosity updates are done in AddExperienceWithCuriosity
	}

	// Update multi-agent system if enabled
	if s.multiAgentRL != nil {
		result := s.multiAgentRL.Update()
		results["multi_agent"] = result
	}

	return results
}

// GetAction gets action from a specific algorithm.
func (s *RLService) GetAction(algorithmName string, state []float32, explore bool) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	alg, ok := s.algorithms[algorithmName]
	if !ok {
		return 0, fmt.Errorf("algorithm not found: %s", algorithmName)
	}

	return alg.GetAction(state, explore), nil
}

// GetStats returns statistics for a specific algorithm.
func (s *RLService) GetStats(algorithmName string) (domainNeural.RLStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	alg, ok := s.algorithms[algorithmName]
	if !ok {
		return domainNeural.RLStats{}, fmt.Errorf("algorithm not found: %s", algorithmName)
	}

	return alg.GetStats(), nil
}

// GetAllStats returns statistics for all algorithms.
func (s *RLService) GetAllStats() map[string]domainNeural.RLStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]domainNeural.RLStats)
	for name, alg := range s.algorithms {
		stats[name] = alg.GetStats()
	}

	if s.multiAgentRL != nil {
		stats["multi_agent"] = s.multiAgentRL.GetStats()
	}

	return stats
}

// Reset resets a specific algorithm.
func (s *RLService) Reset(algorithmName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	alg, ok := s.algorithms[algorithmName]
	if !ok {
		return fmt.Errorf("algorithm not found: %s", algorithmName)
	}

	alg.Reset()
	return nil
}

// ResetAll resets all algorithms.
func (s *RLService) ResetAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, alg := range s.algorithms {
		alg.Reset()
	}

	if s.curiosityModule != nil {
		s.curiosityModule.Reset()
	}

	if s.multiAgentRL != nil {
		s.multiAgentRL.Reset()
	}

	if s.imitationLearner != nil {
		s.imitationLearner.Reset()
	}
}

// SaveCheckpoint saves algorithm state to disk.
func (s *RLService) SaveCheckpoint(algorithmName string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	alg, ok := s.algorithms[algorithmName]
	if !ok {
		return fmt.Errorf("algorithm not found: %s", algorithmName)
	}

	stats := alg.GetStats()

	checkpoint := domainNeural.RLCheckpoint{
		ID:        fmt.Sprintf("%s_%d", algorithmName, time.Now().Unix()),
		Algorithm: stats.Algorithm,
		Stats:     stats,
		CreatedAt: time.Now(),
	}

	if s.checkpointDir == "" {
		return fmt.Errorf("checkpoint directory not configured")
	}

	if err := os.MkdirAll(s.checkpointDir, 0755); err != nil {
		return fmt.Errorf("failed to create checkpoint directory: %w", err)
	}

	filename := filepath.Join(s.checkpointDir, checkpoint.ID+".json")
	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write checkpoint: %w", err)
	}

	return nil
}

// LoadCheckpoint loads algorithm state from disk.
func (s *RLService) LoadCheckpoint(checkpointID string) (*domainNeural.RLCheckpoint, error) {
	if s.checkpointDir == "" {
		return nil, fmt.Errorf("checkpoint directory not configured")
	}

	filename := filepath.Join(s.checkpointDir, checkpointID+".json")
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read checkpoint: %w", err)
	}

	var checkpoint domainNeural.RLCheckpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return nil, fmt.Errorf("failed to unmarshal checkpoint: %w", err)
	}

	return &checkpoint, nil
}

// ListCheckpoints returns available checkpoints.
func (s *RLService) ListCheckpoints() ([]string, error) {
	if s.checkpointDir == "" {
		return nil, fmt.Errorf("checkpoint directory not configured")
	}

	entries, err := os.ReadDir(s.checkpointDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read checkpoint directory: %w", err)
	}

	var checkpoints []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			checkpoints = append(checkpoints, entry.Name()[:len(entry.Name())-5])
		}
	}

	return checkpoints, nil
}

// Curiosity module methods

// GetCuriosityModule returns the curiosity module.
func (s *RLService) GetCuriosityModule() *infraNeural.CuriosityModule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.curiosityModule
}

// EnableCuriosity creates and enables a curiosity module.
func (s *RLService) EnableCuriosity(config domainNeural.CuriosityConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.curiosityModule = infraNeural.NewCuriosityModule(config)
}

// DisableCuriosity removes the curiosity module.
func (s *RLService) DisableCuriosity() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.curiosityModule = nil
}

// ComputeIntrinsicReward computes intrinsic curiosity reward.
func (s *RLService) ComputeIntrinsicReward(state, nextState []float32, action int) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.curiosityModule == nil {
		return 0
	}
	return s.curiosityModule.ComputeIntrinsicReward(state, nextState, action)
}

// Multi-agent methods

// GetMultiAgentRL returns the multi-agent RL system.
func (s *RLService) GetMultiAgentRL() *infraNeural.MultiAgentRL {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.multiAgentRL
}

// EnableMultiAgent creates and enables multi-agent RL.
func (s *RLService) EnableMultiAgent(config domainNeural.MultiAgentConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.multiAgentRL = infraNeural.NewMultiAgentRL(config)
}

// RegisterAgent registers an agent in multi-agent system.
func (s *RLService) RegisterAgent(agentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.multiAgentRL == nil {
		return fmt.Errorf("multi-agent RL not enabled")
	}
	s.multiAgentRL.RegisterAgent(agentID)
	return nil
}

// GetMultiAgentAction gets action for an agent.
func (s *RLService) GetMultiAgentAction(agentID string, state []float32, explore bool) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.multiAgentRL == nil {
		return 0, fmt.Errorf("multi-agent RL not enabled")
	}
	return s.multiAgentRL.GetAction(agentID, state, explore), nil
}

// AddMultiAgentExperience adds experience for an agent.
func (s *RLService) AddMultiAgentExperience(agentID string, exp domainNeural.RLExperience) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.multiAgentRL == nil {
		return fmt.Errorf("multi-agent RL not enabled")
	}
	s.multiAgentRL.AddExperience(agentID, exp)
	return nil
}

// DistributeReward distributes team reward among agents.
func (s *RLService) DistributeReward(teamReward float64) (map[string]float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.multiAgentRL == nil {
		return nil, fmt.Errorf("multi-agent RL not enabled")
	}
	return s.multiAgentRL.DistributeReward(teamReward), nil
}

// Imitation learning methods

// GetImitationLearner returns the imitation learner.
func (s *RLService) GetImitationLearner() *infraNeural.ImitationLearner {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.imitationLearner
}

// EnableImitation creates and enables imitation learning.
func (s *RLService) EnableImitation(config domainNeural.ImitationConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.imitationLearner = infraNeural.NewImitationLearner(config)
	s.algorithms["imitation"] = s.imitationLearner
}

// AddExpertTrajectory adds expert demonstrations.
func (s *RLService) AddExpertTrajectory(states [][]float32, actions []int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.imitationLearner == nil {
		return fmt.Errorf("imitation learning not enabled")
	}
	s.imitationLearner.AddExpertTrajectory(states, actions)
	return nil
}

// Service-level methods

// GetServiceStats returns overall service statistics.
func (s *RLService) GetServiceStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"totalUpdates":     s.totalUpdates,
		"algorithmCount":   len(s.algorithms),
		"uptimeSeconds":    time.Since(s.startTime).Seconds(),
		"hasCuriosity":     s.curiosityModule != nil,
		"hasMultiAgent":    s.multiAgentRL != nil,
		"hasImitation":     s.imitationLearner != nil,
	}
}

// Shutdown gracefully shuts down the service.
func (s *RLService) Shutdown() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Save all checkpoints
	for name := range s.algorithms {
		// Unlock temporarily for SaveCheckpoint
		s.mu.Unlock()
		_ = s.SaveCheckpoint(name)
		s.mu.Lock()
	}

	return nil
}
