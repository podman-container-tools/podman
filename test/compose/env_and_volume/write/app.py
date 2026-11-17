from flask import Flask
import os
app = Flask(__name__)

@app.route('/')
def hello():
    f = open("/data/message", "w")
    f.write(os.getenv("PODMAN_MSG"))
    f.close()
    return "done"

if __name__ == '__main__':
	app.run(host='0.0.0.0')
