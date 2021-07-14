#!/bin/bash

echo 'Running setup_dependencies.sh'

wait_for_curl_success() {
    local healthcheck_url=${1}
    local http_verb=${2}
    local service_name=${3}

    echo "Waiting for [$service_name] to be ready"

    while true; do
        response=$(curl -X $http_verb --write-out %{http_code} --silent --output /dev/null $healthcheck_url)

        if [[ response -eq 200 ]] || [[ response -eq 409 ]]; then
            break
        fi

        echo "[$service_name] not ready ([$response] was the response from curl)"
        sleep 1s
    done

    echo "[$service_name] is ready"
}

wait_for_curl_success "http://localhost:8538/v1/projects/project/topics/eq-submission-topic" "PUT" "pubsub_emulator topic"

echo "Setting up subscriptions to topics..."
curl -X PUT http://localhost:8538/v1/projects/project/subscriptions/rm-receipt-subscription -H 'Content-Type: application/json' -d '{"topic": "projects/project/topics/eq-submission-topic"}'

wait_for_curl_success "http://guest:guest@localhost:17672/api/aliveness-test/%2F" "GET" "rabbitmq"

echo "Containers running and alive"
