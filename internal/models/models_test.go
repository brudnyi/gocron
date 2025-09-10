package models

import "testing"

func TestZeroValues(t *testing.T) {
    _ = CreateJobRequest{}
    _ = Job{}
    _ = JobLog{}
    _ = Webhook{}
}
