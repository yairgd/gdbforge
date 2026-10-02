-- Shared profile / RTOS-awareness helpers for the cortex_r5 scripts (J-Link + Digilent
-- OpenOCD). Install: cp -r lua/mpsoc/cortex_r5 .gdbforge/lua/ — each script dofile()s this
-- one out of its own directory.
--
-- The profile names are the STM32 ones (baremetal, zephyr, freertos) and they ask for the
-- same thing: that the debug server decode the RTOS, so its threads show up in "info
-- threads" and the Threads pane instead of the single hardware context. What is behind
-- them differs, because the RPU is ARMv7-R and both servers were written for Cortex-M:
--
--   openocd + zephyr    works. OpenOCD's rtos/zephyr.c lists cortex_r4 — the target type it
--                       uses for an R5 — next to cortex_m, with the same ARM callee-saved
--                       stacking.
--   openocd + freertos  not available. rtos/FreeRTOS.c matches cortex_m, hla_target and
--                       nds32_v3 only, and "configure -rtos FreeRTOS" against anything else
--                       fails the config stage with "Could not find target in FreeRTOS
--                       compatibility list" — openocd exits before it ever binds the GDB
--                       port. The scripts drop the flag and say so rather than spawn a
--                       server that is going to die.
--   J-Link, either one  the plugin loads and does find the threads, but SEGGER's stock
--                       RTOSPlugin_Zephyr and RTOSPlugin_FreeRTOS unwind a Cortex-M
--                       exception frame — they read XPSR and the EXC_RETURN FPU bit, and
--                       an R5 has neither — so registers and backtraces for any thread
--                       other than the running one cannot be trusted. Wired up anyway,
--                       with that warning printed every time, because the thread list and
--                       its names are still worth having. GDBFORGE_JLINK_RTOS points at
--                       another plugin if you have one that understands ARMv7-R.
--
-- Zephyr also needs the thread metadata compiled in either way: CONFIG_DEBUG_THREAD_INFO=y
-- emits the _kernel_thread_info_* offset tables both servers read. check_profile() looks for
-- them in the ELF and warns up front, rather than leaving it to be discovered when "info
-- threads" comes back with one thread.

local M = {}

M.PROFILES = { "baremetal", "zephyr", "freertos" }

local PROFILE_ALIASES = {
  bare = "baremetal",
  baremetal = "baremetal",
  none = "baremetal",
  off = "baremetal",
  zephyr = "zephyr",
  zephyr_rtos = "zephyr",
  freertos = "freertos",
  free_rtos = "freertos",
  rtos = "freertos",
}

-- OpenOCD RTOS names, keyed by profile. FreeRTOS is absent on purpose: see the header.
local OPENOCD_RTOS = {
  zephyr = "Zephyr",
}

-- SEGGER plugin basenames, keyed by profile (the .so lives beside JLinkGDBServer).
local JLINK_PLUGINS = {
  zephyr = "RTOSPlugin_Zephyr",
  freertos = "RTOSPlugin_FreeRTOS",
}

-- Symbols that have to be in the ELF for the server to find any thread at all.
local PROFILE_SYMBOLS = {
  zephyr = { "_kernel_thread_info_offsets" },
  freertos = { "pxCurrentTCB", "uxTopUsedPriority" },
}

function M.trim(s)
  return (tostring(s or ""):gsub("^%s+", ""):gsub("%s+$", ""))
end

function M.shell_quote(s)
  return "'" .. tostring(s):gsub("'", "'\\''") .. "'"
end

function M.env(name, fallback)
  local v = os.getenv(name)
  if v == nil or M.trim(v) == "" then
    return fallback
  end
  return M.trim(v)
end

function M.expand_tilde(path)
  path = M.trim(path)
  local home = os.getenv("HOME") or ""
  if path == "~" then
    return home
  end
  if path:sub(1, 2) == "~/" then
    return home .. path:sub(2)
  end
  return path
end

local function dirname(path)
  path = tostring(path or ""):gsub("\\", "/")
  local i = path:match("^.*()/")
  if i then
    return path:sub(1, i - 1)
  end
  return "."
end

local function file_exists(path)
  if path == nil or M.trim(path) == "" then
    return false
  end
  return gdbforge.system("test -f " .. M.shell_quote(path)) == 0
end

-- Empty means baremetal, so a script that is called with no profile behaves the way it did
-- before there were any. Returns nil plus the offending word for anything unrecognised.
function M.normalize_profile(arg)
  local p = M.trim(arg):lower():gsub("-", "_")
  if p == "" then
    return "baremetal"
  end
  if PROFILE_ALIASES[p] then
    return PROFILE_ALIASES[p]
  end
  return nil, p
end

-- The OpenAMP scripts take an optional firmware path and an optional profile, in that
-- order, and either can be left out. One argument is a firmware path if such a file is
-- there, and a profile otherwise — the file wins, so a build directory that happens to
-- hold a binary called "zephyr" still means the binary.
function M.parse_fw_and_profile(a1, a2)
  a1 = M.trim(a1)
  a2 = M.trim(a2)

  if a2 ~= "" then
    local p, bad = M.normalize_profile(a2)
    if not p then
      return nil, nil, "unknown profile " .. tostring(bad)
    end
    return a1, p
  end

  if a1 ~= "" then
    if file_exists(a1) then
      return a1, "baremetal"
    end
    local p, bad = M.normalize_profile(a1)
    if p then
      return "", p
    end
    return nil, nil, "no such file, and not a profile: " .. tostring(bad)
  end

  return "", "baremetal"
end

function M.complete_profile(token)
  token = M.trim(token):lower()
  local out = {}
  local seen = {}
  local function add(v)
    if v == "" or seen[v] then
      return
    end
    seen[v] = true
    out[#out + 1] = v
  end
  for _, h in ipairs({ "help", "-h", "--help" }) do
    if token == "" or h:sub(1, #token) == token then
      add(h)
    end
  end
  for _, p in ipairs(M.PROFILES) do
    if token == "" or p:sub(1, #token) == token then
      add(p)
    end
  end
  for alias, canon in pairs(PROFILE_ALIASES) do
    if alias ~= canon and (token == "" or alias:sub(1, #token) == token) then
      add(canon)
    end
  end
  table.sort(out)
  return out
end

function M.register_complete(fn)
  if gdbforge.complete_args then
    gdbforge.complete_args(fn)
  else
    complete_arg = fn
  end
end

-- Tab candidates for a script whose only argument is the profile.
function M.complete_profile_only(token, index)
  if tonumber(index) ~= 1 then
    return {}
  end
  return M.complete_profile(token)
end

-- Tab candidates for the OpenAMP scripts: [firmware] [profile]. Profiles are offered at
-- both positions because a lone profile is a valid call; firmware paths are left to the
-- shell-style completion the user already has in the command line.
function M.complete_fw_and_profile(token, index)
  index = tonumber(index) or 1
  if index == 1 or index == 2 then
    return M.complete_profile(token)
  end
  return {}
end

-- The OpenOCD target to configure. Board cfgs conventionally publish the selected core as
-- _TARGETNAME, and r5_openocd_digilent.cfg does; a cfg of your own that names it something
-- else needs GDBFORGE_OPENOCD_TARGET, or "configure -rtos" lands on an unset Tcl variable
-- and openocd quits during config.
function M.openocd_target()
  return M.env("GDBFORGE_OPENOCD_TARGET", "_TARGETNAME")
end

function M.openocd_rtos(profile)
  return OPENOCD_RTOS[M.normalize_profile(profile) or "baremetal"]
end

-- Extra openocd argv for the profile, plus the RTOS name for the log line (nil when the
-- session is going to run without thread awareness). Passed after -f so the target exists,
-- and still inside the config stage, which is where "configure -rtos" has to happen.
function M.openocd_rtos_args(profile, target)
  profile = M.normalize_profile(profile) or "baremetal"
  local rtos = OPENOCD_RTOS[profile]
  if rtos then
    target = target or M.openocd_target()
    return { "-c", "$" .. target .. " configure -rtos " .. rtos }, rtos
  end
  if profile == "freertos" then
    gdbforge.print("WARNING: OpenOCD has no FreeRTOS support for this core — continuing without it")
    gdbforge.print("  rtos/FreeRTOS.c matches cortex_m, hla_target and nds32_v3; an R5 is")
    gdbforge.print("  cortex_r4 to OpenOCD, and -rtos FreeRTOS would fail the config stage")
    gdbforge.print("  (\"Could not find target in FreeRTOS compatibility list\") before the")
    gdbforge.print("  GDB port opens. Tasks will not appear in info threads.")
    gdbforge.print("  The J-Link scripts do load SEGGER's FreeRTOS plugin — read the caveat")
    gdbforge.print("  in their help() first, it decodes Cortex-M stack frames.")
  end
  return {}, nil
end

-- Resolve the SEGGER RTOS plugin .so for the profile. GDBFORGE_JLINK_RTOS overrides with
-- either a path or a bare plugin name; otherwise look beside JLinkGDBServer, where the
-- Linux tarball keeps them under GDBServer/.
function M.jlink_rtos_plugin(profile, jlink)
  profile = M.normalize_profile(profile) or "baremetal"
  local override = M.env("GDBFORGE_JLINK_RTOS", "")
  local name = JLINK_PLUGINS[profile]
  if override == "" and not name then
    return nil
  end

  local candidates = {}
  local function add(p)
    if p and p ~= "" then
      candidates[#candidates + 1] = p
    end
  end
  local root = dirname(jlink or "")
  if override ~= "" then
    add(M.expand_tilde(override))
    add(M.expand_tilde(override) .. ".so")
    add(root .. "/" .. override)
    add(root .. "/GDBServer/" .. override)
  else
    add(root .. "/GDBServer/" .. name .. ".so")
    add(root .. "/" .. name .. ".so")
  end
  for _, p in ipairs(candidates) do
    if file_exists(p) then
      return p
    end
  end
  return nil, (override ~= "" and override or name)
end

-- Extra JLinkGDBServer argv for the profile, plus a label for the log line.
function M.jlink_rtos_args(profile, jlink)
  profile = M.normalize_profile(profile) or "baremetal"
  if profile == "baremetal" then
    return {}, nil
  end
  local plugin, missing = M.jlink_rtos_plugin(profile, jlink)
  if not plugin then
    gdbforge.print("WARNING: J-Link RTOS plugin not found (" .. tostring(missing) ..
      ") — continuing without thread awareness")
    gdbforge.print("  Looked beside " .. tostring(jlink) .. " and in its GDBServer/ folder.")
    gdbforge.print("  export GDBFORGE_JLINK_RTOS=/path/to/RTOSPlugin.so to point at it.")
    return {}, nil
  end
  gdbforge.print("WARNING: SEGGER's stock RTOS plugins decode Cortex-M stack frames (XPSR,")
  gdbforge.print("  EXC_RETURN), which an ARMv7-R core does not have. The thread list and")
  gdbforge.print("  names are usable; registers and backtraces of threads other than the")
  gdbforge.print("  running one are not. For Zephyr, the OpenOCD scripts here are the")
  gdbforge.print("  accurate path — OpenOCD decodes cortex_r4 threads properly.")
  return { "-rtos", plugin }, plugin
end

-- Look for a symbol without needing a cross-binutils on PATH: readelf reads any ELF
-- whatever architecture it was built for. nil means "could not tell" — no readelf, or no
-- ELF to look at — which is never reported as a problem.
function M.elf_has_symbol(elf, name)
  if not file_exists(elf) then
    return nil
  end
  if gdbforge.system("command -v readelf >/dev/null 2>&1") ~= 0 then
    return nil
  end
  return gdbforge.system("readelf -sW " .. M.shell_quote(elf) ..
    " 2>/dev/null | grep -qw " .. M.shell_quote(name)) == 0
end

-- Preflight for the RTOS profiles: say now what is missing from the image, instead of
-- leaving it to be worked out from an "info threads" that lists one thread. Advisory only
-- — everything here still lets the session start.
function M.check_profile(profile, elf)
  profile = M.normalize_profile(profile) or "baremetal"
  if profile == "baremetal" then
    return true
  end

  for _, sym in ipairs(PROFILE_SYMBOLS[profile] or {}) do
    if M.elf_has_symbol(elf, sym) == false then
      gdbforge.print("WARNING: " .. sym .. " is not in " .. tostring(elf))
      if profile == "zephyr" then
        gdbforge.print("  Zephyr thread awareness needs CONFIG_DEBUG_THREAD_INFO=y — without")
        gdbforge.print("  it the kernel emits no thread offset tables and no server can walk")
        gdbforge.print("  the thread list.")
      elseif sym == "uxTopUsedPriority" then
        gdbforge.print("  FreeRTOS thread awareness needs this symbol kept in the image:")
        gdbforge.print("  configUSE_TRACE_FACILITY=1, or the usual volatile uxTopUsedPriority")
        gdbforge.print("  definition, or the linker drops it and the plugin gives up.")
      else
        gdbforge.print("  Is this really a FreeRTOS image, and built with symbols?")
      end
    end
  end

  if profile == "zephyr" and M.env("ZEPHYR_BASE", "") == "" then
    gdbforge.print("note: ZEPHYR_BASE is unset — thread awareness still works, but GDB gets")
    gdbforge.print("  no kernel source directory, so frames inside the kernel show no source.")
    gdbforge.print("  export ZEPHYR_BASE=~/path/to/zephyr")
  end
  return true
end

-- Source directories for the profile. Run before "target remote", the way the STM32
-- scripts do, so the first stop already resolves.
function M.gdb_setup(profile)
  if (M.normalize_profile(profile) or "baremetal") ~= "zephyr" then
    return
  end
  local zb = M.expand_tilde(M.env("ZEPHYR_BASE", ""))
  if zb ~= "" then
    gdbforge.gdb("dir " .. zb)
  end
  local app = M.expand_tilde(M.env("PWD", ""))
  if app ~= "" then
    gdbforge.gdb("dir " .. app)
  end
end

-- "profile: zephyr  rtos: Zephyr" for the script's own log line.
function M.describe(profile, rtos)
  profile = M.normalize_profile(profile) or "baremetal"
  return "profile: " .. profile .. "  rtos: " .. (rtos and tostring(rtos) or "(none)")
end

function M.profile_help_lines(script, backend)
  gdbforge.print("Profile (optional — default baremetal):")
  gdbforge.print("  baremetal   no RTOS decoding; one thread, the core itself")
  gdbforge.print("  zephyr      Zephyr threads as GDB threads (CONFIG_DEBUG_THREAD_INFO=y)")
  gdbforge.print("  freertos    FreeRTOS tasks as GDB threads")
  gdbforge.print("")
  if backend == "openocd" then
    gdbforge.print("On this core OpenOCD can do zephyr and cannot do freertos: rtos/zephyr.c")
    gdbforge.print("lists cortex_r4, which is the target type an R5 gets, and rtos/FreeRTOS.c")
    gdbforge.print("lists only cortex_m, hla_target and nds32_v3. Asking for freertos prints a")
    gdbforge.print("warning and attaches without thread awareness — passing -rtos FreeRTOS")
    gdbforge.print("would instead kill openocd during config, before the GDB port opens.")
  else
    gdbforge.print("Both profiles load a SEGGER plugin from beside JLinkGDBServer")
    gdbforge.print("(GDBServer/RTOSPlugin_Zephyr.so, GDBServer/RTOSPlugin_FreeRTOS.so), and")
    gdbforge.print("both plugins unwind Cortex-M exception frames — they read XPSR and the")
    gdbforge.print("EXC_RETURN FPU bit, which an ARMv7-R core has neither of. Expect a correct")
    gdbforge.print("thread list with correct names, and do not trust the registers or")
    gdbforge.print("backtrace of a thread that is not the running one. For Zephyr the OpenOCD")
    gdbforge.print("script is the accurate path. GDBFORGE_JLINK_RTOS overrides the plugin.")
  end
  gdbforge.print("")
  gdbforge.print("Examples:")
  gdbforge.print("  :lua " .. script .. " zephyr")
  gdbforge.print("  :lua " .. script .. " baremetal")
end

function M.zephyr_help_lines()
  gdbforge.print("Zephyr (profile zephyr):")
  gdbforge.print("  Build with CONFIG_DEBUG_THREAD_INFO=y — it emits the _kernel_thread_info_*")
  gdbforge.print("  offset tables the debug server walks. The scripts check the ELF for them")
  gdbforge.print("  and warn before attaching.")
  gdbforge.print("  export ZEPHYR_BASE=~/path/to/zephyr for kernel sources (dir $ZEPHYR_BASE")
  gdbforge.print("  and dir $PWD are set for you); threads work without it, sources do not.")
end

return M
