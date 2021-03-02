package watch

import (
	"sync"
	"testing"
	"time"
)

type TimerMaker func(d time.Duration) <-chan time.Time

func ProvideTimerMaker() TimerMaker {
	return func(t time.Duration) <-chan time.Time {
		return time.After(t)
	}
}

type FakeTimerMaker struct {
	RestTimerLock *sync.Mutex
	MaxTimerLock  *sync.Mutex
	t             testing.TB
}

func (f FakeTimerMaker) Maker() TimerMaker {
	return func(d time.Duration) <-chan time.Time {
		var lock *sync.Mutex
		// we have separate locks for the separate uses of timer so that tests can control the timers independently
		switch d {
		case BufferMinRestDuration:
			lock = f.RestTimerLock
		case BufferMaxDuration:
			lock = f.MaxTimerLock
		default:
			// if you hit this, someone (you!?) might have added a new timer with a new duration, and you probably
			// want to add a case above
			f.t.Error("makeTimer called on unsupported duration")
		}
		ret := make(chan time.Time, 1)
		go func() {
			lock.Lock()
			ret <- time.Unix(0, 0)
			lock.Unlock()
			close(ret)
		}()
		return ret
	}
}

func MakeFakeTimerMaker(t testing.TB) FakeTimerMaker {
	restTimerLock := new(sync.Mutex)
	maxTimerLock := new(sync.Mutex)

	return FakeTimerMaker{restTimerLock, maxTimerLock, t}
}
