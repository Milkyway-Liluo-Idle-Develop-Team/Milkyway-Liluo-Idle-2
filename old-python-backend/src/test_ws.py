import json
import time
import requests
import websocket

BASE = "http://localhost:26411"


def http_register(session, username, email, password):
    r = session.post(f"{BASE}/api/register", json={"username": username, "email": email, "password": password})
    return r.json()


def http_login(session, username, password):
    r = session.post(f"{BASE}/api/login", json={"username": username, "password": password})
    return r.json()


def send(ws, payload):
    s = json.dumps(payload, ensure_ascii=False)
    print("C-->", s)
    ws.send(s)


def recv(ws):
    data = ws.recv()
    print("S-->", data)
    return json.loads(data)


def gameplay_test(token, label):
    ws = websocket.create_connection("ws://localhost:26411/ws", header=[f"Cookie: token={token}"])

    # 1. sync
    send(ws, {"type": "sync"})
    r = recv(ws)
    assert r["type"] == "state", r
    print(f"[{label}] sync ok")

    # Ensure starting dialogs 1-5 are unlocked (skip if already unlocked)
    for evt in ["starting_dialog_1", "starting_dialog_2", "starting_dialog_3", "starting_dialog_4", "starting_dialog_5"]:
        send(ws, {"type": "upgrade", "event_id": evt})
        r = recv(ws)
        assert r["type"] in ("state", "error"), r
        if r["type"] == "error":
            assert "already unlocked" in r["message"], r

    # Set queue to felling
    send(ws, {"type": "set_queue", "queue": ["felling_oak_tree"]})
    r = recv(ws)
    assert r["type"] == "state"

    time.sleep(5)

    send(ws, {"type": "sync"})
    r = recv(ws)
    data = r["data"]
    logs = data["inventory"].get("oak_logs", 0)
    print(f"[{label}] oak_logs after 5s: {logs}")
    assert logs >= 2, f"Expected >=2 oak_logs, got {logs}"

    ws.close()
    return logs


def main():
    # Register/login via sessions so cookies are managed automatically
    s1 = requests.Session()
    reg1 = http_register(s1, "user_a", "a@test.com", "123456")
    if not reg1.get("success"):
        reg1 = http_login(s1, "user_a", "123456")
    token1 = s1.cookies.get("token")
    assert token1, "Expected token cookie after login"

    s2 = requests.Session()
    reg2 = http_register(s2, "user_b", "b@test.com", "123456")
    if not reg2.get("success"):
        reg2 = http_login(s2, "user_b", "123456")
    token2 = s2.cookies.get("token")
    assert token2, "Expected token cookie after login"

    # Run gameplay for both users
    logs_a = gameplay_test(token1, "user_a")
    logs_b = gameplay_test(token2, "user_b")

    print(f"\nData isolation check: user_a={logs_a}, user_b={logs_b}")
    assert logs_a >= 2 and logs_b >= 2

    # Wrong token should fail on ws (connection rejected immediately)
    ws_bad = websocket.create_connection("ws://localhost:26411/ws", header=["Cookie: token=invalid_token_xxx"])
    r = recv(ws_bad)
    assert r["type"] == "error", f"Expected error for invalid token, got {r}"
    print("\nInvalid token correctly rejected on WebSocket")
    ws_bad.close()

    # Logout should clear cookie and invalidate session
    logout_resp = s1.post(f"{BASE}/api/logout")
    assert logout_resp.json().get("success") is True
    # after logout, token should be rejected
    ws_logout = websocket.create_connection("ws://localhost:26411/ws", header=[f"Cookie: token={token1}"])
    r = recv(ws_logout)
    assert r["type"] == "error", f"Expected error after logout, got {r}"
    print("\nToken invalidated after logout")
    ws_logout.close()

    print("\nAll tests passed!")


if __name__ == "__main__":
    main()
