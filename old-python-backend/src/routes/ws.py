from routes.ws_handler import handle_ws
from core.auth import require_ws_auth


def register_ws_routes(sock):
    @sock.route("/ws")
    @require_ws_auth
    def ws_route(ws, uid):
        handle_ws(ws, uid)
