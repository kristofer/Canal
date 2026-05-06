#include <stdint.h>
#include <string.h>
#include "esp_err.h"
#include "spi_flash_mmap.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "hal/mmu_hal.h"
#include "hal/cache_ll.h"

// app_main is provided by TinyGo object (kernel_idf.o).
extern void app_main(void);

// Per-domain RAM windows in internal DRAM.
// IDF linker places these in the .dram0.bss section at known fixed addresses.
// Addresses are read after build with xtensa-esp32s3-elf-nm and used as
// --defsym=__domain_ram_origin=0xADDR when linking each domain ELF.
#define DOMAIN_RAM_SLOT_SIZE 16384

__attribute__((aligned(4))) static uint8_t domain_ram_led[DOMAIN_RAM_SLOT_SIZE];
__attribute__((aligned(4))) static uint8_t domain_ram_wifi[DOMAIN_RAM_SLOT_SIZE];
__attribute__((aligned(4))) static uint8_t domain_ram_logger[DOMAIN_RAM_SLOT_SIZE];
__attribute__((aligned(4))) static uint8_t domain_ram_picoceci[DOMAIN_RAM_SLOT_SIZE];
__attribute__((aligned(4))) static uint8_t domain_ram_tls[DOMAIN_RAM_SLOT_SIZE];
__attribute__((aligned(4))) static uint8_t domain_ram_sdcard[DOMAIN_RAM_SLOT_SIZE];

// Return base address of a domain's RAM window.
// Name is matched as a null-terminated C string.
const void *canal_domain_ram(const char *name, uint32_t *size_out)
{
    *size_out = DOMAIN_RAM_SLOT_SIZE;
    if (name[0] == 'l' && name[1] == 'e' && name[2] == 'd' && name[3] == 0)
        return domain_ram_led;
    else if (name[0] == 'w' && name[1] == 'i' && name[2] == 'f' && name[3] == 'i')
        return domain_ram_wifi;
    else if (name[0] == 'l' && name[1] == 'o' && name[2] == 'g')
        return domain_ram_logger;
    else if (name[0] == 'p' && name[1] == 'i' && name[2] == 'c')
        return domain_ram_picoceci;
    else if (name[0] == 't' && name[1] == 'l' && name[2] == 's' && name[3] == 0)
        return domain_ram_tls;
    else if (name[0] == 's' && name[1] == 'd')
        return domain_ram_sdcard;
    *size_out = 0;
    return (void *)0;
}

void canal_bridge_entry(void)
{
    app_main();
}

int32_t canal_flash_read(uint32_t offset, void *out, uint32_t length)
{
    const uint32_t page_size = SPI_FLASH_MMU_PAGE_SIZE;
    const uint32_t map_base = offset & ~(page_size - 1);
    const uint32_t delta = offset - map_base;

    const void *mapped = NULL;
    spi_flash_mmap_handle_t handle = 0;
    esp_err_t err = spi_flash_mmap(map_base, delta + length, SPI_FLASH_MMAP_DATA, &mapped, &handle);
    if (err != ESP_OK)
    {
        return (int32_t)err;
    }

    memcpy(out, (const uint8_t *)mapped + delta, length);
    spi_flash_munmap(handle);
    return (int32_t)ESP_OK;
}

// Map a flash region as executable (IROM) so domain code can run from XIP.
// flash_offset must be page-aligned (IDF will align to 64KB page boundary).
// Returns the virtual base address on success, 0 on failure.
// *handle_out receives an opaque handle that must be passed to canal_munmap_exec
// when the domain exits to free the MMU pages.
uint32_t canal_mmap_exec(uint32_t flash_offset, uint32_t size, uint32_t *handle_out)
{
    const void *mapped = NULL;
    spi_flash_mmap_handle_t handle = 0;
    esp_err_t err = spi_flash_mmap(flash_offset, size, SPI_FLASH_MMAP_INST, &mapped, &handle);
    if (err != ESP_OK)
    {
        *handle_out = 0;
        return 0;
    }
    *handle_out = (uint32_t)handle;
    return (uint32_t)(uintptr_t)mapped;
}

void canal_munmap_exec(uint32_t handle)
{
    if (handle != 0)
        spi_flash_munmap((spi_flash_mmap_handle_t)handle);
}

// Map a file-backed flash segment at an explicit linked virtual address.
// This mirrors the bootloader's fixed-address IROM/DROM mapping model so
// non-PIC TinyGo domain binaries can keep their absolute literals/calls.
int32_t canal_map_flash_segment(uint32_t vaddr, uint32_t flash_offset, uint32_t size)
{
    const uint32_t page_size = SPI_FLASH_MMU_PAGE_SIZE;
    const uint32_t vaddr_aligned = vaddr & ~(page_size - 1);
    const uint32_t paddr_aligned = flash_offset & ~(page_size - 1);
    const uint32_t map_len = (vaddr - vaddr_aligned) + size;
    uint32_t actual_mapped_len = 0;

    mmu_hal_map_region(0, MMU_TARGET_FLASH0, vaddr_aligned, paddr_aligned, map_len, &actual_mapped_len);
    if (actual_mapped_len < map_len)
    {
        return -1;
    }

    cache_bus_mask_t bus_mask = cache_ll_l1_get_bus(0, vaddr_aligned, map_len);
    cache_ll_l1_enable_bus(0, bus_mask);
#if !CONFIG_FREERTOS_UNICORE
    bus_mask = cache_ll_l1_get_bus(1, vaddr_aligned, map_len);
    cache_ll_l1_enable_bus(1, bus_mask);
#endif
    return 0;
}

//

// Create a FreeRTOS task from a raw code entry address.
// Doing the function-pointer cast in C avoids TinyGo ABI edge cases when
// passing function pointers directly into xTaskCreate from Go.
int32_t canal_create_task(uint32_t entry,
                          const char *name,
                          uint32_t stack_words,
                          void *params,
                          uint32_t priority,
                          TaskHandle_t *out_handle)
{
    return (int32_t)xTaskCreatePinnedToCore(
        (TaskFunction_t)(uintptr_t)entry,
        name,
        stack_words,
        params,
        priority,
        out_handle,
        tskNO_AFFINITY);
}
