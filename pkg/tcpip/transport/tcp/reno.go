// Copyright 2018 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tcp

// renoState stores the variables related to TCP New Reno congestion
// control algorithm.
//
// +stateify savable
type renoState struct {
	s *sender
}

// newRenoCC initializes the state for the NewReno congestion control algorithm.
func newRenoCC(s *sender) *renoState {
	return &renoState{s: s}
}

// updateSlowStart will update the congestion window as per the slow-start
// algorithm used by NewReno. If after adjusting the congestion window
// we cross the SSthreshold then it will return the number of packets that
// must be consumed in congestion avoidance mode.
func (r *renoState) updateSlowStart(packetsAcked int) int {
	// Don't let the congestion window cross into the congestion
	// avoidance range.
	newcwnd := r.s.sndCwnd + packetsAcked
	if newcwnd >= r.s.sndSsthresh {
		newcwnd = r.s.sndSsthresh
		r.s.sndCAAckCount = 0
	}

	packetsAcked -= newcwnd - r.s.sndCwnd
	r.s.sndCwnd = newcwnd
	return packetsAcked
}

// updateCongestionAvoidance will update congestion window in congestion
// avoidance mode as described in RFC5681 section 3.1
func (r *renoState) updateCongestionAvoidance(packetsAcked int) {
	// Consume the packets in congestion avoidance mode.
	r.s.sndCAAckCount += packetsAcked
	if r.s.sndCAAckCount >= r.s.sndCwnd {
		r.s.sndCwnd += r.s.sndCAAckCount / r.s.sndCwnd
		r.s.sndCAAckCount = r.s.sndCAAckCount % r.s.sndCwnd
	}
}

// reduceSlowStartThreshold reduces the slow-start threshold per RFC 5681,
// page 6, eq. 4. It is called when we detect congestion in the network.
func (r *renoState) reduceSlowStartThreshold() {
	r.s.sndSsthresh = r.s.outstanding / 2
	if r.s.sndSsthresh < 2 {
		r.s.sndSsthresh = 2
	}

}

// Update updates the congestion state based on the number of packets that
// were acknowledged.
// Update implements congestionControl.Update.
func (r *renoState) Update(packetsAcked int) {
	if r.s.sndCwnd < r.s.sndSsthresh {
		packetsAcked = r.updateSlowStart(packetsAcked)
		if packetsAcked == 0 {
			return
		}
	}
	r.updateCongestionAvoidance(packetsAcked)
}

// HandleNDupAcks implements congestionControl.HandleNDupAcks.
func (r *renoState) HandleNDupAcks() {
	// A retransmit was triggered due to nDupAckThreshold
	// being hit. Reduce our slow start threshold.
	r.reduceSlowStartThreshold()
}

// HandleRTOExpired implements congestionControl.HandleRTOExpired.
func (r *renoState) HandleRTOExpired() {
	// We lost a packet, so reduce ssthresh.
	r.reduceSlowStartThreshold()

	// Reduce the congestion window to 1, i.e., enter slow-start. Per
	// RFC 5681, page 7, we must use 1 regardless of the value of the
	// initial congestion window.
	r.s.sndCwnd = 1
}
