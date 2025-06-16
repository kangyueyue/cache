package retry

import "time"

// RetryConfig defines the configuration for retrying operations.
type RetryConfig struct {
	MaxAttempts uint          // 最大尝试次数
	Delay       time.Duration // 重试间隔
	RetryIfFn   func(error) bool
}

// NewRetryConfig creates a new RetryConfig with the provided options.
func NewRetryConfig(opts ...Option) *RetryConfig {
	r := DefaultConfig

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// DefaultConfig is the default configuration for retrying operations.
var DefaultConfig = &RetryConfig{
	MaxAttempts: 3,
	Delay:       1000,
	RetryIfFn: func(err error) bool {
		return err != nil
	},
}

type Option func(*RetryConfig)

func WithMaxAttempts(maxAttempts uint) Option {
	return func(c *RetryConfig) {
		c.MaxAttempts = maxAttempts
	}
}

func WithDelay(delay time.Duration) Option {
	return func(c *RetryConfig) {
		c.Delay = delay
	}
}
