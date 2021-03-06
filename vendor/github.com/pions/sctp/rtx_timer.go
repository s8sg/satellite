package sctp

import (
	//"fmt"
	"math"
	"sync"
	"time"
)

const (
	rtoInitial     float64 = 3.0 * 1000  // msec
	rtoMin         float64 = 1.0 * 1000  // msec
	rtoMax         float64 = 60.0 * 1000 // msec
	rtoAlpha       float64 = 0.125
	rtoBeta        float64 = 0.25
	maxInitRetrans uint    = 8
	pathMaxRetrans uint    = 5
)

// rtoManager manages Rtx timeout values.
// This is an implementation of RFC 4960 sec 6.3.1.
type rtoManager struct {
	srtt   float64
	rttvar float64
	rto    float64
	mutex  sync.RWMutex
}

// newRTOManager creates a new rtoManager.
func newRTOManager() *rtoManager {
	return &rtoManager{
		rto: rtoInitial,
	}
}

// setNewRTT takes a newly measured RTT then adjust the RTO in msec.
func (m *rtoManager) setNewRTT(rtt float64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.srtt == 0 {
		// First measurement
		m.srtt = rtt
		m.rttvar = rtt / 2
	} else {
		// Subsequent rtt measurement
		m.rttvar = (1-rtoBeta)*m.rttvar + rtoBeta*(math.Abs(m.srtt-rtt))
		m.srtt = (1-rtoAlpha)*m.srtt + rtoAlpha*rtt
	}
	m.rto = math.Min(math.Max(m.srtt+4*m.rttvar, rtoMin), rtoMax)
}

// getRTO simply returns the current RTO in msec.
func (m *rtoManager) getRTO() float64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.rto
}

// reset resets the RTO variables to the initial values.
func (m *rtoManager) reset() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.srtt = 0
	m.rttvar = 0
	m.rto = rtoInitial
}

// rtxTimerObserver is the inteface to a timer observer.
// NOTE: Observers MUST NOT call start() or stop() method on rtxTimer
// from within these callbacks.
type rtxTimerObserver interface {
	onRetransmissionTimeout(timerID int, n uint)
	onRetransmissionFailure(timerID int)
}

// rtxTimer provides the retnransmission timer conforms with RFC 4960 Sec 6.3.1
type rtxTimer struct {
	id         int
	observer   rtxTimerObserver
	maxRetrans uint
	stopFunc   stopTimerLoop
	mutex      sync.RWMutex
}

type stopTimerLoop func()

// newRTXTimer creates a new retransmission timer.
func newRTXTimer(id int, observer rtxTimerObserver, maxRetrans uint) *rtxTimer {
	return &rtxTimer{
		id:         id,
		observer:   observer,
		maxRetrans: maxRetrans,
	}
}

// start starts the timer.
func (t *rtxTimer) start(rto float64) bool {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	// If there's a previous timer running, stop it.
	if t.stopFunc != nil {
		return false
	}

	// Note: rto value is intentionally not capped by RTO.Min to allow
	// fast timeout for the tests. Non-test code should pass in the
	// rto generated by rtoManager getRTO() method which caps the
	// value at RTO.Min or at RTO.Max.
	var nRtos uint

	cancelCh := make(chan struct{})

	go func() {
		canceling := false

		for !canceling {
			timeout := calculateNextTimeout(rto, nRtos)
			timer := time.NewTimer(time.Duration(timeout) * time.Millisecond)

			select {
			case <-timer.C:
				nRtos++
				if nRtos <= t.maxRetrans {
					t.observer.onRetransmissionTimeout(t.id, nRtos)
				} else {
					t.stop()
					t.observer.onRetransmissionFailure(t.id)
				}
			case <-cancelCh:
				canceling = true
				timer.Stop()
			}
		}
	}()

	t.stopFunc = func() {
		close(cancelCh)
	}

	return true
}

// stop stops the timer.
func (t *rtxTimer) stop() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.stopFunc != nil {
		t.stopFunc()
		t.stopFunc = nil
	}
}

// isRunning tests if the timer is running.
// Debug purpose only
func (t *rtxTimer) isRunning() bool {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	return (t.stopFunc != nil)
}

func calculateNextTimeout(rto float64, nRtos uint) float64 {
	m := 1 << nRtos
	return math.Min(rto*float64(m), rtoMax)
}
