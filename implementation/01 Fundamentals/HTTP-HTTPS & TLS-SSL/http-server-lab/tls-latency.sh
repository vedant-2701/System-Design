for i in 1 2 3; do
    { time echo "Q" | timeout 10 openssl s_client -connect google.com:443 -quiet 2>/dev/null; } 2>&1 | grep real
done