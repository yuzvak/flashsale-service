package clock

import (
	"time"
)

type Clock interface {
	Now() time.Time
	Since(t time.Time) time.Duration
	Until(t time.Time) time.Duration
	Sleep(d time.Duration)
}

type RealClock struct{}

func NewRealClock() *RealClock {
	return &RealClock{}
}

func (c *RealClock) Now() time.Time {
	return time.Now().UTC()
}

func (c *RealClock) Since(t time.Time) time.Duration {
	return time.Since(t)
}

func (c *RealClock) Until(t time.Time) time.Duration {
	return time.Until(t)
}

func (c *RealClock) Sleep(d time.Duration) {
	time.Sleep(d)
}

type MockClock struct {
	currentTime time.Time
}

func NewMockClock(t time.Time) *MockClock {
	return &MockClock{
		currentTime: t,
	}
}

func (c *MockClock) Now() time.Time {
	return c.currentTime
}

func (c *MockClock) Since(t time.Time) time.Duration {
	return c.currentTime.Sub(t)
}

func (c *MockClock) Until(t time.Time) time.Duration {
	return t.Sub(c.currentTime)
}

func (c *MockClock) Sleep(d time.Duration) {
}

func (c *MockClock) Advance(d time.Duration) {
	c.currentTime = c.currentTime.Add(d)
}

func (c *MockClock) Set(t time.Time) {
	c.currentTime = t
}
