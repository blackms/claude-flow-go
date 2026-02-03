// Package neural provides domain types for neural learning systems.
package neural

import (
	"time"
)

// RLAlgorithm represents a reinforcement learning algorithm type.
type RLAlgorithm string

const (
	// AlgorithmPPO is Proximal Policy Optimization.
	AlgorithmPPO RLAlgorithm = "ppo"
	// AlgorithmDQN is Deep Q-Network.
	AlgorithmDQN RLAlgorithm = "dqn"
	// AlgorithmA2C is Advantage Actor-Critic.
	AlgorithmA2C RLAlgorithm = "a2c"
	// AlgorithmQLearning is tabular Q-Learning.
	AlgorithmQLearning RLAlgorithm = "q-learning"
	// AlgorithmSARSA is State-Action-Reward-State-Action.
	AlgorithmSARSA RLAlgorithm = "sarsa"
	// AlgorithmDecisionTransformer is Decision Transformer.
	AlgorithmDecisionTransformer RLAlgorithm = "decision-transformer"
	// AlgorithmCuriosity is Curiosity-driven exploration.
	AlgorithmCuriosity RLAlgorithm = "curiosity"
	// AlgorithmMultiAgent is Multi-Agent RL.
	AlgorithmMultiAgent RLAlgorithm = "multi-agent"
	// AlgorithmImitation is Imitation Learning.
	AlgorithmImitation RLAlgorithm = "imitation"
)

// RLConfig is the base configuration for RL algorithms.
type RLConfig struct {
	// Algorithm is the algorithm type.
	Algorithm RLAlgorithm `json:"algorithm"`

	// LearningRate for gradient updates.
	LearningRate float64 `json:"learningRate"`

	// Gamma is the discount factor.
	Gamma float64 `json:"gamma"`

	// EntropyCoef is the entropy bonus coefficient.
	EntropyCoef float64 `json:"entropyCoef"`

	// ValueLossCoef is the value loss coefficient.
	ValueLossCoef float64 `json:"valueLossCoef"`

	// MaxGradNorm is the maximum gradient norm for clipping.
	MaxGradNorm float64 `json:"maxGradNorm"`

	// Epochs is the number of training epochs per update.
	Epochs int `json:"epochs"`

	// MiniBatchSize is the mini-batch size for training.
	MiniBatchSize int `json:"miniBatchSize"`

	// NumActions is the number of discrete actions.
	NumActions int `json:"numActions"`

	// InputDim is the input state dimension.
	InputDim int `json:"inputDim"`

	// HiddenDim is the hidden layer dimension.
	HiddenDim int `json:"hiddenDim"`
}

// DefaultRLConfig returns the default RL configuration.
func DefaultRLConfig() RLConfig {
	return RLConfig{
		Algorithm:     AlgorithmPPO,
		LearningRate:  0.0003,
		Gamma:         0.99,
		EntropyCoef:   0.01,
		ValueLossCoef: 0.5,
		MaxGradNorm:   0.5,
		Epochs:        4,
		MiniBatchSize: 64,
		NumActions:    4,
		InputDim:      256,
		HiddenDim:     64,
	}
}

// PPOConfig is the configuration for PPO algorithm.
type PPOConfig struct {
	RLConfig

	// ClipRange is the clipping parameter for policy ratio.
	ClipRange float64 `json:"clipRange"`

	// ClipRangeVF is the clipping parameter for value function (nil = no clipping).
	ClipRangeVF *float64 `json:"clipRangeVf,omitempty"`

	// TargetKL is the target KL divergence for early stopping.
	TargetKL float64 `json:"targetKL"`

	// GAELambda is the lambda for Generalized Advantage Estimation.
	GAELambda float64 `json:"gaeLambda"`
}

// DefaultPPOConfig returns the default PPO configuration.
func DefaultPPOConfig() PPOConfig {
	return PPOConfig{
		RLConfig:  DefaultRLConfig(),
		ClipRange: 0.2,
		TargetKL:  0.01,
		GAELambda: 0.95,
	}
}

// DQNConfig is the configuration for DQN algorithm.
type DQNConfig struct {
	RLConfig

	// BufferSize is the replay buffer size.
	BufferSize int `json:"bufferSize"`

	// ExplorationInitial is the initial exploration rate.
	ExplorationInitial float64 `json:"explorationInitial"`

	// ExplorationFinal is the final exploration rate.
	ExplorationFinal float64 `json:"explorationFinal"`

	// ExplorationDecay is the number of steps for exploration decay.
	ExplorationDecay int `json:"explorationDecay"`

	// TargetUpdateFreq is the frequency of target network updates.
	TargetUpdateFreq int `json:"targetUpdateFreq"`

	// DoubleDQN enables Double DQN.
	DoubleDQN bool `json:"doubleDQN"`

	// DuelingNetwork enables Dueling architecture.
	DuelingNetwork bool `json:"duelingNetwork"`
}

// DefaultDQNConfig returns the default DQN configuration.
func DefaultDQNConfig() DQNConfig {
	config := DefaultRLConfig()
	config.Algorithm = AlgorithmDQN
	config.LearningRate = 0.0001

	return DQNConfig{
		RLConfig:           config,
		BufferSize:         10000,
		ExplorationInitial: 1.0,
		ExplorationFinal:   0.01,
		ExplorationDecay:   10000,
		TargetUpdateFreq:   100,
		DoubleDQN:          true,
		DuelingNetwork:     false,
	}
}

// A2CConfig is the configuration for A2C algorithm.
type A2CConfig struct {
	RLConfig

	// NSteps is the number of steps for N-step returns.
	NSteps int `json:"nSteps"`

	// UseGAE enables Generalized Advantage Estimation.
	UseGAE bool `json:"useGAE"`

	// GAELambda is the lambda for GAE.
	GAELambda float64 `json:"gaeLambda"`
}

// DefaultA2CConfig returns the default A2C configuration.
func DefaultA2CConfig() A2CConfig {
	config := DefaultRLConfig()
	config.Algorithm = AlgorithmA2C
	config.LearningRate = 0.0007
	config.Epochs = 1

	return A2CConfig{
		RLConfig:  config,
		NSteps:    5,
		UseGAE:    true,
		GAELambda: 0.95,
	}
}

// QLearningConfig is the configuration for Q-Learning algorithm.
type QLearningConfig struct {
	RLConfig

	// ExplorationInitial is the initial exploration rate.
	ExplorationInitial float64 `json:"explorationInitial"`

	// ExplorationFinal is the final exploration rate.
	ExplorationFinal float64 `json:"explorationFinal"`

	// ExplorationDecay is the number of steps for exploration decay.
	ExplorationDecay int `json:"explorationDecay"`

	// MaxStates is the maximum number of states in Q-table.
	MaxStates int `json:"maxStates"`

	// UseEligibilityTraces enables eligibility traces.
	UseEligibilityTraces bool `json:"useEligibilityTraces"`

	// TraceDecay is the eligibility trace decay rate.
	TraceDecay float64 `json:"traceDecay"`
}

// DefaultQLearningConfig returns the default Q-Learning configuration.
func DefaultQLearningConfig() QLearningConfig {
	config := DefaultRLConfig()
	config.Algorithm = AlgorithmQLearning
	config.LearningRate = 0.1
	config.Epochs = 1
	config.MiniBatchSize = 1

	return QLearningConfig{
		RLConfig:             config,
		ExplorationInitial:   1.0,
		ExplorationFinal:     0.01,
		ExplorationDecay:     10000,
		MaxStates:            10000,
		UseEligibilityTraces: false,
		TraceDecay:           0.9,
	}
}

// SARSAConfig is the configuration for SARSA algorithm.
type SARSAConfig struct {
	RLConfig

	// ExplorationInitial is the initial exploration rate.
	ExplorationInitial float64 `json:"explorationInitial"`

	// ExplorationFinal is the final exploration rate.
	ExplorationFinal float64 `json:"explorationFinal"`

	// ExplorationDecay is the number of steps for exploration decay.
	ExplorationDecay int `json:"explorationDecay"`

	// MaxStates is the maximum number of states in Q-table.
	MaxStates int `json:"maxStates"`

	// UseExpectedSARSA enables Expected SARSA.
	UseExpectedSARSA bool `json:"useExpectedSARSA"`

	// UseEligibilityTraces enables eligibility traces.
	UseEligibilityTraces bool `json:"useEligibilityTraces"`

	// TraceDecay is the eligibility trace decay rate.
	TraceDecay float64 `json:"traceDecay"`
}

// DefaultSARSAConfig returns the default SARSA configuration.
func DefaultSARSAConfig() SARSAConfig {
	config := DefaultRLConfig()
	config.Algorithm = AlgorithmSARSA
	config.LearningRate = 0.1
	config.Epochs = 1
	config.MiniBatchSize = 1

	return SARSAConfig{
		RLConfig:             config,
		ExplorationInitial:   1.0,
		ExplorationFinal:     0.01,
		ExplorationDecay:     10000,
		MaxStates:            10000,
		UseExpectedSARSA:     false,
		UseEligibilityTraces: false,
		TraceDecay:           0.9,
	}
}

// CuriosityConfig is the configuration for Curiosity-driven exploration.
type CuriosityConfig struct {
	RLConfig

	// IntrinsicCoef is the intrinsic reward coefficient.
	IntrinsicCoef float64 `json:"intrinsicCoef"`

	// ForwardLR is the forward model learning rate.
	ForwardLR float64 `json:"forwardLR"`

	// InverseLR is the inverse model learning rate.
	InverseLR float64 `json:"inverseLR"`

	// FeatureDim is the feature dimension.
	FeatureDim int `json:"featureDim"`

	// UseRND enables Random Network Distillation.
	UseRND bool `json:"useRND"`

	// NoveltyThreshold is the threshold for novelty detection.
	NoveltyThreshold float64 `json:"noveltyThreshold"`
}

// DefaultCuriosityConfig returns the default Curiosity configuration.
func DefaultCuriosityConfig() CuriosityConfig {
	config := DefaultRLConfig()
	config.Algorithm = AlgorithmCuriosity

	return CuriosityConfig{
		RLConfig:         config,
		IntrinsicCoef:    0.01,
		ForwardLR:        0.001,
		InverseLR:        0.001,
		FeatureDim:       64,
		UseRND:           false,
		NoveltyThreshold: 0.1,
	}
}

// ImitationConfig is the configuration for Imitation Learning.
type ImitationConfig struct {
	RLConfig

	// BehavioralCloning enables pure behavioral cloning.
	BehavioralCloning bool `json:"behavioralCloning"`

	// DAgger enables Dataset Aggregation.
	DAgger bool `json:"dagger"`

	// ExpertWeight is the weight for expert actions.
	ExpertWeight float64 `json:"expertWeight"`

	// MaxExpertTrajectories is the maximum expert trajectories to store.
	MaxExpertTrajectories int `json:"maxExpertTrajectories"`
}

// DefaultImitationConfig returns the default Imitation configuration.
func DefaultImitationConfig() ImitationConfig {
	config := DefaultRLConfig()
	config.Algorithm = AlgorithmImitation

	return ImitationConfig{
		RLConfig:              config,
		BehavioralCloning:     true,
		DAgger:                false,
		ExpertWeight:          0.5,
		MaxExpertTrajectories: 1000,
	}
}

// MultiAgentConfig is the configuration for Multi-Agent RL.
type MultiAgentConfig struct {
	RLConfig

	// NumAgents is the number of agents.
	NumAgents int `json:"numAgents"`

	// SharedReward enables shared reward distribution.
	SharedReward bool `json:"sharedReward"`

	// IndependentLearners enables independent learning.
	IndependentLearners bool `json:"independentLearners"`

	// CommunicationEnabled enables agent communication.
	CommunicationEnabled bool `json:"communicationEnabled"`

	// CoordinationBonus is the bonus for coordination.
	CoordinationBonus float64 `json:"coordinationBonus"`
}

// DefaultMultiAgentConfig returns the default Multi-Agent configuration.
func DefaultMultiAgentConfig() MultiAgentConfig {
	config := DefaultRLConfig()
	config.Algorithm = AlgorithmMultiAgent

	return MultiAgentConfig{
		RLConfig:             config,
		NumAgents:            4,
		SharedReward:         true,
		IndependentLearners:  true,
		CommunicationEnabled: false,
		CoordinationBonus:    0.1,
	}
}

// RLExperience represents a single experience tuple.
type RLExperience struct {
	// State is the current state embedding.
	State []float32 `json:"state"`

	// Action is the action taken.
	Action int `json:"action"`

	// ActionString is the action as string.
	ActionString string `json:"actionString,omitempty"`

	// Reward is the reward received.
	Reward float64 `json:"reward"`

	// NextState is the next state embedding.
	NextState []float32 `json:"nextState"`

	// Done indicates if this is a terminal state.
	Done bool `json:"done"`

	// Value is the value estimate (for actor-critic).
	Value float64 `json:"value,omitempty"`

	// LogProb is the log probability of the action.
	LogProb float64 `json:"logProb,omitempty"`

	// Advantage is the advantage estimate.
	Advantage float64 `json:"advantage,omitempty"`

	// Return is the return (cumulative reward).
	Return float64 `json:"return,omitempty"`

	// Timestamp is when the experience was created.
	Timestamp time.Time `json:"timestamp"`
}

// RLUpdateResult represents the result of an RL update.
type RLUpdateResult struct {
	// PolicyLoss is the policy loss.
	PolicyLoss float64 `json:"policyLoss"`

	// ValueLoss is the value loss.
	ValueLoss float64 `json:"valueLoss"`

	// Entropy is the policy entropy.
	Entropy float64 `json:"entropy"`

	// TotalLoss is the combined loss.
	TotalLoss float64 `json:"totalLoss"`

	// ClipFraction is the fraction of clipped updates (PPO).
	ClipFraction float64 `json:"clipFraction,omitempty"`

	// ApproxKL is the approximate KL divergence.
	ApproxKL float64 `json:"approxKL,omitempty"`

	// TDError is the TD error (Q-learning, SARSA).
	TDError float64 `json:"tdError,omitempty"`

	// Epsilon is the exploration rate.
	Epsilon float64 `json:"epsilon,omitempty"`

	// Accuracy is the action prediction accuracy.
	Accuracy float64 `json:"accuracy,omitempty"`

	// LatencyMs is the update latency in milliseconds.
	LatencyMs float64 `json:"latencyMs"`
}

// RLStats contains statistics for an RL algorithm.
type RLStats struct {
	// Algorithm is the algorithm type.
	Algorithm RLAlgorithm `json:"algorithm"`

	// UpdateCount is the number of updates performed.
	UpdateCount int64 `json:"updateCount"`

	// BufferSize is the current buffer size.
	BufferSize int `json:"bufferSize"`

	// Epsilon is the current exploration rate.
	Epsilon float64 `json:"epsilon"`

	// AvgLoss is the average loss.
	AvgLoss float64 `json:"avgLoss"`

	// AvgReward is the average reward.
	AvgReward float64 `json:"avgReward"`

	// AvgLatencyMs is the average update latency.
	AvgLatencyMs float64 `json:"avgLatencyMs"`

	// QTableSize is the Q-table size (for tabular methods).
	QTableSize int `json:"qTableSize,omitempty"`

	// StepCount is the total number of steps.
	StepCount int64 `json:"stepCount"`

	// LastUpdate is when the last update occurred.
	LastUpdate time.Time `json:"lastUpdate"`
}

// RLCheckpoint represents a saved algorithm state.
type RLCheckpoint struct {
	// ID is the checkpoint identifier.
	ID string `json:"id"`

	// Algorithm is the algorithm type.
	Algorithm RLAlgorithm `json:"algorithm"`

	// Weights are the network weights.
	Weights map[string][][]float64 `json:"weights"`

	// Config is the algorithm configuration.
	Config interface{} `json:"config"`

	// Stats are the algorithm statistics.
	Stats RLStats `json:"stats"`

	// CreatedAt is when the checkpoint was created.
	CreatedAt time.Time `json:"createdAt"`
}

// RLAlgorithmInterface defines the interface for RL algorithms.
type RLAlgorithmInterface interface {
	// AddExperience adds experience to the algorithm.
	AddExperience(experience RLExperience)

	// Update performs a learning update.
	Update() RLUpdateResult

	// GetAction returns an action for the given state.
	GetAction(state []float32, explore bool) int

	// GetStats returns algorithm statistics.
	GetStats() RLStats

	// Reset resets the algorithm state.
	Reset()
}
