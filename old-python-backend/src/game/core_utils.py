"""Shared utility functions used across game modules."""

from __future__ import annotations

from typing import Any


def to_float(value: Any, default: float = 0.0) -> float:
    try:
        return float(value)
    except (TypeError, ValueError):
        return default


def clamp(value: float, low: float, high: float) -> float:
    return max(low, min(high, value))


def compare(actual: float, expected: float, comp: str | None) -> bool:
    if comp is None:
        return actual >= expected
    comp = str(comp).lower()
    if comp == "bigger":
        return actual > expected
    if comp == "equal":
        return actual == expected
    if comp == "smaller":
        return actual < expected
    if comp == "bigger_or_equal":
        return actual >= expected
    if comp == "smaller_or_equal":
        return actual <= expected
    return actual >= expected
