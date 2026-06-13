cd $HOME/linux-basics

mkdir -p /tmp/myapp-test

go build -o server ./cmd/server/ 2>&1 && echo "Build OK" && ls -lh server

APP_PORT=9090 APP_LOG_FILE=/tmp/myapp-test/myapp.log ./server &
SERVER_PID=$!
sleep 1

echo "=== /health ==="
curl -s http://localhost:9090/health | python3 -m json.tool

echo ""
echo "=== /work ==="
curl -s "http://localhost:9090/work?n=100000"

echo ""
echo "=== /log ==="
curl -s "http://localhost:9090/log?level=error&msg=test-error"

echo ""
echo "=== Log file contents ==="
cat /tmp/myapp-test/myapp.log

echo ""
echo "=== Graceful shutdown ==="
kill -15 $SERVER_PID
wait $SERVER_PID 2>/dev/null
echo "Exit code: $?"

echo ""
echo "=== Final log (should show shutdown) ==="
cat /tmp/myapp-test/myapp.log