=== picoceci over TCP ===

[picoceci] Ready v0.1.0-dev
  tip: type '---' to enter/exit paste mode for multi-line programs

> ---
(paste mode on: type '---' to run)
... "sd_list_test.pc"
... | entries: Any |
... 
... Console println: 'SD_LIST_BEGIN'.
... entries := FS list: '/'.
... 
... entries do: [ :e |
...     Console println: e printString
... ].
... 
... Console println: 'SD_LIST_DONE'.
... 
... ---
error: list "/" failed: filesystem service not available (shim not initialized)

