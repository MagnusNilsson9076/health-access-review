#!/bin/sh
set -eu

curl --fail-with-body \
  --request POST \
  --header 'Content-Type: application/json' \
  --data '{"appointments":[{"patient_ref":"P-1042","clinic":"North Clinic","scheduled_at":"2026-10-03 09:30","status":"rescheduled"},{"patient_ref":"P-1043","clinic":"North Clinic","scheduled_at":"2026-10-03 10:00","status":"chart-reviewed"}]}' \
  http://localhost:8080/reviews
