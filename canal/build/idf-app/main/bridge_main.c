#include <stdint.h>

// app_main is provided by TinyGo object (kernel_idf.o).
extern void app_main(void);

void canal_bridge_entry(void) {
    app_main();
}
