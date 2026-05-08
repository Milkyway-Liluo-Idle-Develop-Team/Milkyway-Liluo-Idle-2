import json
import requests
import websocket

BASE = "http://localhost:26411"


def http_register(session, username, email, password):
    r = session.post(f"{BASE}/api/register", json={"username": username, "email": email, "password": password})
    return r.json()


def http_login(session, username, password):
    r = session.post(f"{BASE}/api/login", json={"username": username, "password": password})
    return r.json()


def main():
    s = requests.Session()
    reg = http_register(s, "single_test_user", "single@test.com", "123456")
    if not reg.get("success"):
        reg = http_login(s, "single_test_user", "123456")
    token = s.cookies.get("token")
    assert token, "Expected token cookie after login"

    # Open first connection
    ws1 = websocket.create_connection("ws://localhost:26411/ws", header=[f"Cookie: token={token}"])
    ws1.send(json.dumps({"type": "sync"}, ensure_ascii=False))
    r1 = json.loads(ws1.recv())
    assert r1["type"] == "state", f"Expected state, got {r1}"
    print("[ws1] First connection established, sync ok")

    # Open second connection with same token
    ws2 = websocket.create_connection("ws://localhost:26411/ws", header=[f"Cookie: token={token}"])

    # ws1 should receive kick-off message
    kick_msg = json.loads(ws1.recv())
    assert kick_msg["type"] == "error", f"Expected error on ws1, got {kick_msg}"
    assert "Another connection has been established" in kick_msg["message"], f"Unexpected message: {kick_msg}"
    print("[ws1] Kicked off correctly:", kick_msg["message"])

    # ws2 should work normally
    ws2.send(json.dumps({"type": "sync"}, ensure_ascii=False))
    r2 = json.loads(ws2.recv())
    assert r2["type"] == "state", f"Expected state on ws2, got {r2}"
    print("[ws2] Second connection works, sync ok")

    # ws1 should be closed (recv should raise or return empty)
    try:
        data = ws1.recv()
        if data:
            assert False, f"ws1 should be closed, got: {data}"
    except (websocket.WebSocketConnectionClosedException, ConnectionResetError, ConnectionAbortedError):
        pass
    print("[ws1] Connection is closed as expected")

    ws1.close()
    ws2.close()
    print("\nSingle WS connection test passed!")


if __name__ == "__main__":
    main()
