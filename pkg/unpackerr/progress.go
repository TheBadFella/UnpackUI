package unpackerr

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/bytefmt"
	"golift.io/xtractr"
)

const (
	minimumProgressInterval = time.Second
	defaultProgressInterval = 15 * time.Second
	noProgressText          = "no progress yet"
)

// ExtractProgress holds the progress for an entire Extract.
// An Extract is "a new item in a watch folder" or "a download in a starr app".
// Either may produce multiple xtractr.XFile structs (extractable archives).
type ExtractProgress struct {
	*xtractr.Progress
	// Extract that exists in the map.
	*Extract
	// Number of archives in this Extract.
	Archives int
	// Number of archives extracted from this Extract.
	Extracted int
	// StartedAt is when the current archive began extracting.
	StartedAt time.Time
	// UpdatedAt is when the current progress counters were last refreshed.
	UpdatedAt time.Time
}

func (p *ExtractProgress) String() string {
	if p == nil || p.Progress == nil {
		return noProgressText
	}

	wrote, total := p.Bytes()

	return fmt.Sprintf("on archive: %d/%d @ %sB/%sB (%.0f%%): %s",
		p.Extracted+1, p.Archives, bytefmt.ByteSize(wrote), bytefmt.ByteSize(total),
		p.Percent(), strings.TrimLeft(strings.TrimPrefix(p.XFile.FilePath, p.Path), string(filepath.Separator)))
}

func (p *ExtractProgress) Bytes() (uint64, uint64) {
	if p == nil {
		return 0, 0
	}

	return progressBytes(p.Progress)
}

func (p *ExtractProgress) Speed(now time.Time) (uint64, bool) {
	if p == nil || p.Progress == nil || p.StartedAt.IsZero() {
		return 0, false
	}

	elapsed := p.UpdatedAt.Sub(p.StartedAt)
	if elapsed <= 0 && now.After(p.StartedAt) {
		elapsed = now.Sub(p.StartedAt)
	}

	if elapsed < time.Second {
		return 0, false
	}

	wrote, _ := p.Bytes()
	if wrote == 0 {
		return 0, false
	}

	speed := uint64(float64(wrote) / elapsed.Seconds())

	return speed, speed > 0
}

func (p *ExtractProgress) ETA(now time.Time) (time.Duration, bool) {
	speed, ok := p.Speed(now)
	if !ok {
		return 0, false
	}

	wrote, total := p.Bytes()
	if total == 0 || wrote >= total {
		return 0, false
	}

	remaining := total - wrote

	eta := time.Duration(float64(remaining) / float64(speed) * float64(time.Second)).Round(time.Second)
	if eta > 0 && eta < time.Second {
		eta = time.Second
	}

	return eta, eta > 0
}

func (u *Unpackerr) progressUpdateCallback(item *Extract) func(xtractr.Progress) {
	return func(prog xtractr.Progress) { // sends update to u.handleProgress() (below)
		u.progChan <- &ExtractProgress{Progress: &prog, Extract: item}
	}
}

// exp = what just came in, it's ephemeral.
// exp.Progress = also what just came in, must set it here.
// exp.XProg = what is saved in the map, update this one.
func (u *Unpackerr) handleProgress(exp *ExtractProgress) {
	if exp == nil || exp.Extract == nil || exp.XProg == nil || exp.Progress == nil {
		return
	}

	xprog := exp.XProg
	now := time.Now()

	if xprog.Progress != nil && xprog.XFile != exp.XFile {
		xprog.Extracted++
		xprog.StartedAt = now
	} else if xprog.StartedAt.IsZero() {
		xprog.StartedAt = exp.progressStartedAt(now)
	}

	xprog.Progress = exp.Progress
	xprog.UpdatedAt = now
}

func (u *Unpackerr) printProgress(now time.Time) {
	for name, data := range u.Map {
		if data.Status != EXTRACTING {
			continue
		}

		if prog := data.XProg.String(); prog != noProgressText {
			u.Printf("[%s] Status: %s (%v, elapsed: %v) %s", data.App, name, data.Status.Desc(),
				now.Sub(data.Updated).Round(time.Second), prog)
		}
	}
}

func (p *ExtractProgress) progressStartedAt(now time.Time) time.Time {
	if p != nil && p.Extract != nil && p.Resp != nil && !p.Resp.Started.IsZero() &&
		!p.Resp.Started.After(now) {
		return p.Resp.Started
	}

	if p != nil && p.Extract != nil && p.Status == EXTRACTING && !p.Updated.IsZero() &&
		!p.Updated.After(now) {
		return p.Updated
	}

	return now
}

func progressBytes(progress *xtractr.Progress) (uint64, uint64) {
	if progress == nil {
		return 0, 0
	}

	if progress.Total > 0 {
		return progress.Wrote, progress.Total
	}

	if progress.Compressed > 0 {
		return progress.Read, progress.Compressed
	}

	return 0, 0
}
