import logging
import os

import flask
from flask_cors import CORS
from flask_sock import Sock

from models import database
from routes.api import api_bp
from routes.ws import register_ws_routes

DEFAULT_HOST = "localhost"
DEFAULT_PORT = 26411

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s: %(message)s",
)


def create_app():
    app = flask.Flask("server")
    CORS(app, supports_credentials=True)
    sock = Sock(app)

    app.register_blueprint(api_bp)
    register_ws_routes(sock)

    @app.teardown_appcontext
    def teardown_db(exception):
        database.close_db()

    return app


def main():
    database.init_db()
    app = create_app()
    host = os.environ.get("HOST", DEFAULT_HOST)
    port = int(os.environ.get("PORT", DEFAULT_PORT))
    app.run(host=host, port=port)


if __name__ == "__main__":
    main()
