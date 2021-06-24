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

wait_for_curl_success "http://localhost:8539/v1/projects/project/topics/eq-submission-topic" "PUT" "pubsub_emulator topic"
wait_for_curl_success "http://localhost:8539/v1/projects/offline-project/topics/offline-receipt-topic" "PUT" "pubsub_emulator topic"
wait_for_curl_success "http://localhost:8539/v1/projects/ppo-undelivered-project/topics/ppo-undelivered-mail-topic" "PUT" "pubsub_emulator topic"
wait_for_curl_success "http://localhost:8539/v1/projects/qm-undelivered-project/topics/qm-undelivered-mail-topic" "PUT" "pubsub_emulator topic"
wait_for_curl_success "http://localhost:8539/v1/projects/fulfilment-confirmed-project/topics/fulfilment-confirmed-topic" "PUT" "pubsub_emulator topic"
wait_for_curl_success "http://localhost:8539/v1/projects/eq-fulfilment-project/topics/eq-fulfilment-topic" "PUT" "pubsub_emulator topic"

echo "Setting up subscriptions to topics..."
curl -X PUT http://localhost:8539/v1/projects/project/subscriptions/rm-receipt-subscription -H 'Content-Type: application/json' -d '{"topic": "projects/project/topics/eq-submission-topic"}'
curl -X PUT http://localhost:8539/v1/projects/offline-project/subscriptions/rm-offline-receipt-subscription -H 'Content-Type: application/json' -d '{"topic": "projects/offline-project/topics/offline-receipt-topic"}'
curl -X PUT http://localhost:8539/v1/projects/ppo-undelivered-project/subscriptions/rm-ppo-undelivered-subscription -H 'Content-Type: application/json' -d '{"topic": "projects/ppo-undelivered-project/topics/ppo-undelivered-mail-topic"}'
curl -X PUT http://localhost:8539/v1/projects/qm-undelivered-project/subscriptions/rm-qm-undelivered-subscription -H 'Content-Type: application/json' -d '{"topic": "projects/qm-undelivered-project/topics/qm-undelivered-mail-topic"}'
curl -X PUT http://localhost:8539/v1/projects/fulfilment-confirmed-project/subscriptions/fulfilment-confirmed-subscription -H 'Content-Type: application/json' -d '{"topic": "projects/fulfilment-confirmed-project/topics/fulfilment-confirmed-topic"}'
curl -X PUT http://localhost:8539/v1/projects/eq-fulfilment-project/subscriptions/eq-fulfilment-subscription -H 'Content-Type: application/json' -d '{"topic": "projects/eq-fulfilment-project/topics/eq-fulfilment-topic"}'

wait_for_curl_success "http://guest:guest@localhost:17672/api/aliveness-test/%2F" "GET" "rabbitmq"

echo "Containers running and alive"
