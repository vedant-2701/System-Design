cd $HOME/tcp-labs/go

echo "=== Go vet ==="
go vet ./...

echo "=== Go build server ==="
go build ./server/...

echo "=== Go build client ==="
go build ./client/...

echo "=== All tests ==="
go test ./... -v -timeout 30s 2>&1 | grep -E "(PASS|FAIL|RUN|ok)"