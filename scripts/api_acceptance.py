#!/usr/bin/env python3
"""Checks a live `gdbforge -serve` endpoint against a loaded target (J-Link, OpenOCD,
gdbserver, native): interrupt reliability, target_running errors, clean console output, batch.

    GDBFORGE_TOKEN=T scripts/api_acceptance.py [http://127.0.0.1:7777] [bp_location]

bp_location (file.c:123), if given, must be in code the target runs often: the script sets it
and checks that continue stops there with the right bkpt_number.

Delete other breakpoints first: a breakpoint hit counts as a stop, so with one in the way the
target never keeps running (and batch, correctly, leaves it stopped).
"""
import json
import os
import random
import re
import sys
import time
import urllib.error
import urllib.request

base = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:7777"
bp = sys.argv[2] if len(sys.argv) > 2 else ""
token = os.environ.get("GDBFORGE_TOKEN")
if not token:
    sys.exit("set GDBFORGE_TOKEN")


def call(tool, args=None):
    req = urllib.request.Request(
        f"{base}/api/{tool}", data=json.dumps(args or {}).encode(),
        headers={"Authorization": f"Bearer {token}"})
    try:
        with urllib.request.urlopen(req, timeout=60) as r:
            return json.load(r)
    except urllib.error.HTTPError as e:  # errors carry a JSON body too
        return json.load(e)


failed = False


def check(name, ok, detail=""):
    global failed
    print(("PASS " if ok else "FAIL ") + name + ("" if ok else f": {detail}"))
    failed = failed or not ok


call("interrupt")  # start from a known state: stopped

good = 0
for _ in range(20):
    call("continue", {"wait_ms": 0})
    time.sleep(random.uniform(0, 0.5))
    r = call("interrupt")
    good += r.get("state") == "stopped" and bool(r.get("pc"))
check("1. interrupt returns stopped with a pc, 20/20", good == 20, f"{good}/20")

if bp:
    ins = call("gdb_command", {"command": f"-break-insert {bp}", "format": "mi"}).get("result", "")
    m = re.search(r'bkpt=\{number="(\d+)"', ins)
    r = call("continue", {"wait_ms": 5000})
    check("2. continue stops at the breakpoint with its number",
          m is not None and r.get("state") == "stopped" and str(r.get("bkpt_number")) == m.group(1), r)
    if m:
        call("gdb_command", {"command": f"-break-delete {m.group(1)}"})

call("continue", {"wait_ms": 0})
r = call("read_memory", {"address": "$sp", "length": 4})
check("3. a command that needs a halt while running gives target_running",
      r.get("error") == "target_running", r)
call("interrupt")

line = call("gdb_command", {"command": "info registers pc"}).get("result", "")
check("4. info registers pc is one clean line", "\n" not in line and line.startswith("pc"), repr(line))

call("continue", {"wait_ms": 0})
op = {"tool": "read_memory", "args": {"address": "$sp", "length": 16}}
r = call("batch", {"halt": True, "resume": True, "ops": [op] * 5})
results = (r.get("result") or {}).get("results", [])
check("5. batch of 5 reads in one call, target running afterwards",
      len(results) == 5 and all("error" not in x for x in results) and r.get("state") == "running", r)
call("interrupt")

sys.exit(1 if failed else 0)
