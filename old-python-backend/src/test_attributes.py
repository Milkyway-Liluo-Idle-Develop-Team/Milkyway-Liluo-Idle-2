"""属性系统 + PlayerContext + 战斗 + 概率取整 单元测试。"""

import sys
import os
import json
import time
import math

sys.path.insert(0, os.path.join(os.path.dirname(__file__)))

from game.attributes import Modifier, AttributeSet
from game.settlement import _resolve_quantity, _probabilistic_round, _effective_loop_time
from game.combat import simulate_combat, CombatResult


# ===========================================================================
# 1. AttributeSet 单元测试
# ===========================================================================

def test_empty():
    a = AttributeSet({"hp": 100.0, "atk": 10.0}, [])
    assert a.get("hp") == 100.0
    assert a.get("atk") == 10.0
    assert a.get("missing") == 0.0
    print("[PASS] test_empty")


def test_flat_only():
    mods = [
        Modifier("atk", 5.0, "flat"),
        Modifier("atk", 3.0, "flat"),
        Modifier("def", 10.0, "flat"),
    ]
    a = AttributeSet({"atk": 10.0}, mods)
    assert a.get("atk") == 18.0
    assert a.get("def") == 10.0
    print("[PASS] test_flat_only")


def test_percent_add():
    mods = [
        Modifier("atk", 0.2, "percent_add"),
        Modifier("atk", 0.3, "percent_add"),
    ]
    a = AttributeSet({"atk": 100.0}, mods)
    assert abs(a.get("atk") - 150.0) < 1e-9
    print("[PASS] test_percent_add")


def test_percent_mult():
    mods = [
        Modifier("atk", 0.1, "percent_mult"),
        Modifier("atk", 0.2, "percent_mult"),
    ]
    a = AttributeSet({"atk": 100.0}, mods)
    expected = 100.0 * 1.1 * 1.2
    assert abs(a.get("atk") - expected) < 1e-9
    print("[PASS] test_percent_mult")


def test_full_formula():
    mods = [
        Modifier("atk", 20.0, "flat"),
        Modifier("atk", 0.5, "percent_add"),
        Modifier("atk", 0.1, "percent_mult"),
        Modifier("atk", 0.2, "percent_mult"),
    ]
    a = AttributeSet({"atk": 100.0}, mods)
    expected = (100.0 + 20.0) * (1.0 + 0.5) * 1.1 * 1.2
    assert abs(a.get("atk") - expected) < 1e-9
    print("[PASS] test_full_formula")


def test_no_base():
    mods = [Modifier("new_attr", 50.0, "flat")]
    a = AttributeSet({}, mods)
    assert a.get("new_attr") == 50.0
    print("[PASS] test_no_base")


def test_negative_modifier():
    mods = [Modifier("hatred_multiplier", -0.2, "flat")]
    a = AttributeSet({}, mods)
    assert abs(a.get("hatred_multiplier") - (-0.2)) < 1e-9
    print("[PASS] test_negative_modifier")


def test_to_dict():
    mods = [Modifier("atk", 10.0, "flat")]
    a = AttributeSet({"hp": 100.0}, mods)
    d = a.to_dict()
    assert d["hp"] == 100.0
    assert d["atk"] == 10.0
    d["hp"] = 999
    assert a.get("hp") == 100.0
    print("[PASS] test_to_dict")


def test_performance():
    import time as _time
    mods = []
    for i in range(50):
        mods.append(Modifier(f"attr_{i % 10}", float(i), "flat"))
        if i % 3 == 0:
            mods.append(Modifier(f"attr_{i % 10}", 0.1, "percent_add"))
    base = {f"attr_{i}": 100.0 for i in range(10)}
    start = _time.perf_counter()
    for _ in range(10000):
        a = AttributeSet(base, mods)
        for j in range(10):
            a.get(f"attr_{j}")
    elapsed = _time.perf_counter() - start
    assert elapsed < 2.0, f"Performance too slow: {elapsed:.3f}s"
    print(f"[PASS] test_performance ({elapsed:.3f}s for 10k builds + 100k gets)")


# ===========================================================================
# 2. 概率取整单元测试
# ===========================================================================

def test_probabilistic_round_integer():
    """整数不受影响。"""
    assert _probabilistic_round(5.0) == 5
    assert _probabilistic_round(0.0) == 0
    print("[PASS] test_probabilistic_round_integer")


def test_probabilistic_round_distribution():
    """大量采样验证概率分布。3.7 应大约 70% 为 4，30% 为 3。"""
    import random
    random.seed(42)
    results = [_probabilistic_round(3.7) for _ in range(10000)]
    fours = sum(1 for r in results if r == 4)
    ratio = fours / 10000
    assert 0.65 < ratio < 0.75, f"Expected ~0.7, got {ratio}"
    print(f"[PASS] test_probabilistic_round_distribution (ratio={ratio:.3f})")


def test_probabilistic_round_negative_frac():
    """值为整数（无正小数部分）时直接返回。"""
    assert _probabilistic_round(3.0) == 3
    print("[PASS] test_probabilistic_round_negative_frac")


# ===========================================================================
# 3. 战斗模拟单元测试（用 mock PlayerContext）
# ===========================================================================

class MockAttrSet:
    def __init__(self, d):
        self._d = d
    def get(self, key, default=0.0):
        return self._d.get(key, default)
    def to_dict(self):
        return dict(self._d)

class MockSkill:
    def __init__(self, level=1, exp=0.0):
        self.level = level
        self.exp = exp


class MockCtx:
    def __init__(self, attrs):
        self.attr_set = MockAttrSet(attrs)
        self.skills = {}

    def get_event_count(self, event_id: str) -> int:
        return 0


def test_combat_basic():
    """基本战斗计算。"""
    ctx = MockCtx({
        "physical_damage": 10.0,
        "defense": 0.0,
        "accuracy": 10.0,
        "attack_interval": 2.0,
    })
    enemy = {"hp": 50, "attack": 5, "defense": 2, "attack_interval": 3}
    result = simulate_combat(ctx, enemy)
    assert result.victory is True
    # player_dmg = max(1, 10-2) = 8, hits = ceil(50/8) = 7, duration = 7*2 = 14
    assert abs(result.duration - 14.0) < 1e-9
    print("[PASS] test_combat_basic")


def test_combat_high_defense():
    """敌人防御高于玩家攻击时，伤害至少为 1。"""
    ctx = MockCtx({
        "physical_damage": 3.0,
        "defense": 0.0,
        "attack_interval": 1.0,
    })
    enemy = {"hp": 10, "attack": 1, "defense": 100}
    result = simulate_combat(ctx, enemy)
    assert result.victory is True
    # player_dmg = max(1, 3-100) = 1, hits = 10, duration = 10*1 = 10
    assert abs(result.duration - 10.0) < 1e-9
    print("[PASS] test_combat_high_defense")


def test_combat_one_shot():
    """一击必杀。"""
    ctx = MockCtx({
        "physical_damage": 999.0,
        "defense": 0.0,
        "attack_interval": 0.5,
    })
    enemy = {"hp": 10, "attack": 1, "defense": 0}
    result = simulate_combat(ctx, enemy)
    assert result.victory is True
    # player_dmg = 999, hits = 1, duration = max(0.5, 0.5) = 0.5
    assert abs(result.duration - 0.5) < 1e-9
    print("[PASS] test_combat_one_shot")


# ===========================================================================
# 4. _resolve_quantity 单元测试（用 mock）
# ===========================================================================

def test_resolve_quantity_no_modifiers():
    """无修饰时返回原始值。"""
    ctx = MockCtx({})
    result = _resolve_quantity(5.0, ctx, "felling")
    assert result == 5
    print("[PASS] test_resolve_quantity_no_modifiers")


def test_resolve_quantity_with_mult():
    """乘算修饰（production_multiplier）。"""
    ctx = MockCtx({"felling_production_multiplier": 1.0})  # +100%
    # base=5, * (1+1.0) * level_multiplier(1)=1.0 = 10.0
    result = _resolve_quantity(5.0, ctx, "felling")
    assert result == 10
    print("[PASS] test_resolve_quantity_with_mult")


def test_resolve_quantity_with_flat():
    """加算修饰。"""
    ctx = MockCtx({"felling_reward_flat": 3.0})
    import random
    random.seed(42)
    # base=5 + 3.0 = 8.0
    result = _resolve_quantity(5.0, ctx, "felling")
    assert result == 8
    print("[PASS] test_resolve_quantity_with_flat")


def test_resolve_quantity_fractional():
    """小数部分按概率取整验证。"""
    ctx = MockCtx({"reward_mult": 0.5})  # +50%
    # base=3, * 1.5 = 4.5 → 概率取整
    import random
    random.seed(42)
    results = [_resolve_quantity(3.0, ctx, None) for _ in range(10000)]
    fives = sum(1 for r in results if r == 5)
    ratio = fives / 10000
    assert 0.45 < ratio < 0.55, f"Expected ~0.5, got {ratio}"
    print(f"[PASS] test_resolve_quantity_fractional (ratio={ratio:.3f})")


# ===========================================================================
# 5. 集成测试（需要服务器运行）
# ===========================================================================
SERVER_URL = "http://localhost:26411"
WS_URL = "ws://localhost:26411/ws"


def _register_user(username: str) -> str:
    import urllib.request
    data = json.dumps({
        "username": username,
        "email": f"{username}@test.com",
        "password": "test123",
    }).encode()
    req = urllib.request.Request(
        f"{SERVER_URL}/api/register",
        data=data,
        headers={"Content-Type": "application/json"},
    )
    resp = urllib.request.urlopen(req)
    cookie = resp.headers.get("Set-Cookie", "")
    for part in cookie.split(";"):
        if part.strip().startswith("token="):
            return part.strip().split("=", 1)[1]
    body = json.loads(resp.read().decode())
    return body.get("token", "")


def _ws_send_recv(ws, msg: dict) -> dict:
    ws.send(json.dumps(msg))
    raw = ws.recv()
    return json.loads(raw)


def test_integration_state_fields():
    import websocket as ws_mod
    username = f"test_fields_{int(time.time())}"
    token = _register_user(username)
    ws = ws_mod.create_connection(WS_URL, cookie=f"token={token}")
    try:
        resp = _ws_send_recv(ws, {"type": "gameplay_light"})
        assert resp["type"] == "gameplay_light", f"Expected gameplay_light, got {resp['type']}"
        data = resp["data"]
        for field in ["inventory", "skills", "unlocked_events",
                       "queue_items", "queue_index", "queue_progress_seconds",
                       "equipment", "tools", "attributes"]:
            assert field in data, f"Missing field: {field}"
        assert data["equipment"] == {}
        assert data["tools"] == {}
        assert isinstance(data.get("attributes"), dict)
        print("[PASS] test_integration_state_fields")
    finally:
        ws.close()


def test_integration_settle():
    import websocket as ws_mod
    username = f"test_settle_{int(time.time())}"
    token = _register_user(username)
    ws = ws_mod.create_connection(WS_URL, cookie=f"token={token}")
    try:
        for i in range(1, 6):
            resp = _ws_send_recv(ws, {"type": "upgrade", "event_id": f"starting_dialog_{i}"})
            assert resp["type"] == "delta", f"Dialog {i} failed: {resp}"

        resp = _ws_send_recv(ws, {"type": "set_queue", "queue": ["felling_oak_tree"]})
        assert resp["type"] == "delta"

        time.sleep(5)
        resp = _ws_send_recv(ws, {"type": "sync"})
        assert resp["type"] == "delta"
        patch = resp["data"].get("patch", {})
        oak_logs = patch.get("inventory", {}).get("oak_logs", 0)
        assert oak_logs >= 2, f"Expected oak_logs >= 2, got {oak_logs}"
        print(f"[PASS] test_integration_settle (oak_logs={oak_logs})")
    finally:
        ws.close()


# ===========================================================================
# 主入口
# ===========================================================================
if __name__ == "__main__":
    print("=== AttributeSet 单元测试 ===")
    test_empty()
    test_flat_only()
    test_percent_add()
    test_percent_mult()
    test_full_formula()
    test_no_base()
    test_negative_modifier()
    test_to_dict()
    test_performance()

    print("\n=== 概率取整单元测试 ===")
    test_probabilistic_round_integer()
    test_probabilistic_round_distribution()
    test_probabilistic_round_negative_frac()

    print("\n=== 战斗模拟单元测试 ===")
    test_combat_basic()
    test_combat_high_defense()
    test_combat_one_shot()

    print("\n=== 奖励修饰单元测试 ===")
    test_resolve_quantity_no_modifiers()
    test_resolve_quantity_with_mult()
    test_resolve_quantity_with_flat()
    test_resolve_quantity_fractional()

    print("\n=== 所有单元测试通过 ===\n")

    try:
        import urllib.request
        urllib.request.urlopen(f"{SERVER_URL}/", timeout=2)
        server_running = True
    except Exception:
        server_running = False

    if server_running:
        print("=== 集成测试（服务器已运行）===")
        test_integration_state_fields()
        test_integration_settle()
        print("\n=== 集成测试全部通过 ===")
    else:
        print("[SKIP] 集成测试跳过（服务器未运行）")

    print("\n全部完成!")
