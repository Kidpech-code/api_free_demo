#!/usr/bin/env bash
BASE_URL=${BASE_URL:-http://localhost:8080}
EMAIL=${EMAIL:-demo+curl@kidpech.app}
PASSWORD=${PASSWORD:-Passw0rd!}

set -euo pipefail

echo "Registering ${EMAIL}"
REGISTER_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"email":"'"$EMAIL"'","password":"'"$PASSWORD"'","name":"Curl Demo"}')

echo "Logging in"
TOKENS=$(curl -s -X POST "$BASE_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"'"$EMAIL"'","password":"'"$PASSWORD"'"}')
ACCESS=$(echo "$TOKENS" | jq -r '.tokens.access_token')
REFRESH=$(echo "$TOKENS" | jq -r '.tokens.refresh_token')

echo "Creating profile"
curl -s -X POST "$BASE_URL/api/v1/profiles" \
  -H "Authorization: Bearer $ACCESS" \
  -H "Content-Type: application/json" \
  -d '{"first_name":"Curl","last_name":"Demo","bio":"Created from curl script"}' | jq

echo "Listing profiles"
curl -s -X GET "$BASE_URL/api/v1/profiles?limit=5" -H "Authorization: Bearer $ACCESS" | jq

echo "Refreshing token"
curl -s -X POST "$BASE_URL/api/v1/auth/refresh" -H "Content-Type: application/json" \
  -d '{"refresh_token":"'"$REFRESH"'"}' | jq
