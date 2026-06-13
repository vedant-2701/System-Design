echo "--- Saving TLS Session ---"

echo "Q" | openssl s_client \
    -connect google.com:443 \
    -sess_out /home/vedant/http-server/tls_session.pem \
    2>&1

echo "--- Second connection using saved session ---"

echo "Q" | openssl s_client \
    -connect google.com:443 \
    -sess_in /home/vedant/http-server/tls_session.pem \
    2>&1 | grep -iE "Reused|New,|Session-ID|TLS session ticket|Cipher"
