include ../../Makefile.Common

e2e-test:
	make -C ../../ otelcontribcol
	go test -v --tags=e2e .

# Generate the mock files
.PHONY: mockgen
mockgen:
	go install go.uber.org/mock/mockgen@latest
	mkdir -p ./mock
	mockgen -package mock -source=./supervisor/commander/commander.go -destination ./supervisor/commander/mock/commander.go
	mockgen -package mock -source=./supervisor/healthchecker/healthchecker.go -destination ./supervisor/healthchecker/mock/healthchecker.go
	mockgen -package mock -source=./supervisor/supervisor.go -destination ./supervisor/mock/supervisor.go


