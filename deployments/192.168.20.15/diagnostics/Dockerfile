FROM weknora-app-builder:feedback-names-20260923
WORKDIR /app
COPY --from=project go.mod go.sum ./
COPY --from=project internal ./internal
COPY --from=project third_party ./third_party
COPY wecom_ack_probe_test.go internal/im/wecom/ack_diagnostic_test.go
RUN --mount=type=cache,target=/go/pkg/mod --network=none go test ./internal/im/wecom -run 'TestDiagnosticRejectedFinish|TestWeComFeedback' -count=1 -v
