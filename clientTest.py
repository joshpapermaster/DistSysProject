from websocket import create_connection
import json
from timeit import default_timer as timer
ws = create_connection("wss://osavxvy2eg.execute-api.us-east-1.amazonaws.com/dev")
print ("Sending 'Hello, World'...")
payload = json.dumps({"action": "onMessage", "message": "Hello, world"})
start = timer()
ws.send(payload)
print ("Sent")
print ("Receiving...")
result =  ws.recv()
end = timer()
print("Received" + result)
diff = end - start
print(str(diff) + " milliseconds")
ws.close()
