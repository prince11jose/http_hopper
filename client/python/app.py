from flask import Flask, request, jsonify
import argparse

app = Flask(__name__)

@app.route('/', methods=['GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'OPTIONS'])
def handle_traffic():
    # Get the request method
    method = request.method

    # Get the request headers
    headers = dict(request.headers)

    # Get the request body (if any)
    data = request.get_json(silent=True) or request.data.decode('utf-8')

    # Log the received traffic details
    print(f"\nReceived {method} request:")
    print(f"Headers: {headers}")
    print(f"Body: {data}")

    # Respond with success
    return jsonify({"status": "success", "message": f"Received {method} request"}), 200

if __name__ == "__main__":
    # Set up command-line argument parsing
    parser = argparse.ArgumentParser(description='Start the Flask application.')
    parser.add_argument('--port', type=int, default=8080, help='Port to run the Flask app on (default: 8080)')

    args = parser.parse_args()

    # Start the Flask application
    app.run(host='0.0.0.0', port=args.port)
