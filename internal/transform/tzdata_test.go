package transform

// Embed the timezone database in the test binary so golden timestamps render
// with the correct Europe/Paris offset regardless of the host's zoneinfo.
import _ "time/tzdata"
