#!/bin/sh

if [ -z "$1" ]
then
    echo "required argument: input json file"
    exit
fi

if [ -z "$2" ]
then
    echo "required argument: emulator base url"
    exit
fi

if [ -z "$3" ]
then
    echo "required argument: pubsub topic name"
    exit
fi

payload='{"messages": []}'

for message in $(cat $1 | jq -r '.[] | @base64'); do
    _jq() {
     echo "${message}" | base64 -d | jq -r "${1}"
    }

    # if you are having issues with jq when running pubsub-init, try the following line instead
    # payloadData=$(_jq .)
    payloadData=$(_jq)

    echo | base64 -w0 > /dev/null 2>&1
    if [ $? -eq 0 ]; then
      # GNU coreutils base64, '-w' supported
      payloadData=$(echo -n $payloadData | base64 -w 0)
    else
      # Openssl base64, no wrapping by default
      payloadData=$(echo -n $payloadData | base64)
    fi

    payload=$(echo $payload | jq --arg data "$payloadData" '.messages += [{"data": $data}]')
done

curl -v http://$2:8085/v1/projects/golang-template/topics/$3:publish -H 'content-type: application/json' --data "$payload"