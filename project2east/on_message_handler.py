import json
import boto3
import os

dynamodb = boto3.client('dynamodb')


def handle(event, context):
    value = [""]
    audience = ""
    type=""
    try:
        audience = json.loads(event['body'])['audience']
        type = json.loads(event['body'])['type']
        value = json.loads(event['body'])['value']
    except:
        audience = ""
        type = ""
        value = [""]
    senderConnectionId = event['requestContext']['connectionId']
    if type == "Introduction":
        value.append(senderConnectionId)

    data = {
        "Type": type,
        "Value": value,
    }
    data = json.dumps(data)
    paginator = dynamodb.get_paginator('scan')

    connectionIds = []

    apigatewaymanagementapi = boto3.client('apigatewaymanagementapi',
    endpoint_url = "https://" + event["requestContext"]["domainName"] + "/" + event["requestContext"]["stage"])

    # Retrieve all connectionIds from the database
    for page in paginator.paginate(TableName=os.environ['SOCKET_CONNECTIONS_TABLE_NAME']):
        connectionIds.extend(page['Items'])

    # Emit the recieved value to all the connected devices
    for connectionId in connectionIds:
        if audience == "all" or audience == connectionId['connectionId']['S']:
            apigatewaymanagementapi.post_to_connection(
                Data = data,
                ConnectionId=connectionId['connectionId']['S']
            )

    return {}
