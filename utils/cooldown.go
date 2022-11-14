package utils

import "time"

type Cooldown struct {
	minWait time.Duration
	maxWait time.Duration
	valWait time.Duration

	tick time.Time
}

func NewCooldown(min, max time.Duration) *Cooldown {
	return &Cooldown{
		minWait: min,
		maxWait: max,
		valWait: min,
		tick:    time.Now(),
	}
}

func (cd *Cooldown) Increase(step time.Duration) {
	cd.valWait = cd.valWait + step
	if cd.valWait > cd.maxWait {
		cd.valWait = cd.maxWait
	}
	if cd.valWait < cd.minWait {
		cd.valWait = cd.minWait
	}
}
func (cd *Cooldown) Set(val time.Duration) {
	cd.valWait = val
}
func (cd *Cooldown) Reset() {
	cd.valWait = cd.minWait
}
func (cd *Cooldown) Tick() {
	cd.tick = time.Now()
}
func (cd *Cooldown) WaitDuration() time.Duration {
	dur := cd.valWait - time.Since(cd.tick)
	if dur < 0 {
		return 0
	}
	return dur
}
func (cd *Cooldown) Wait() {
	<-time.After(cd.WaitDuration())
}
