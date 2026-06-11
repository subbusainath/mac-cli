from mac_orchestrator.db import error_hash, normalize_error, to_vector_literal


def test_normalize_strips_volatile_parts():
    a = normalize_error('File "/tmp/x/app.py", line 42, in handler\n  ValueError: bad input 0x7f3a')
    b = normalize_error('File "/tmp/y/app.py", line 99, in handler\n  ValueError: bad input 0x55ee')
    assert a == b


def test_error_hash_stable_and_hex():
    h1 = error_hash("ValueError: boom")
    h2 = error_hash("ValueError: boom")
    assert h1 == h2
    assert len(h1) == 64
    assert int(h1, 16)  # valid hex


def test_vector_literal_format():
    assert to_vector_literal([0.1, -1.0, 2.5]) == "[0.1,-1.0,2.5]"
