package postgres

import "testing"

func TestZeroValues(t *testing.T) {
    _ = CreateJobLogParams{}
    _ = CreateJobParams{}
    _ = GetJobLogsParams{}
    _ = Job{}
    _ = JobLog{}
    _ = NullJobStatus{}
    _ = Queries{}
    _ = Store{}
    _ = UpdateJobAfterExecutionParams{}
    _ = UpdateJobStatusParams{}
}
