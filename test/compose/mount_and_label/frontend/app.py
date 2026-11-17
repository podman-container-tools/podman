from flask import Flask
app = Flask(__name__)

@app.route('/')
def hello():
    f = open("/data/message")
    return f.read()

if __name__ == '__main__':
	app.run(host='0.0.0.0')
