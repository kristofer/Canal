#include <stdint.h>
#include <string.h>
#include "esp_err.h"
#include "esp_heap_caps.h"
#include "esp_log.h"
#include "spi_flash_mmap.h"
#include "esp_wifi.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "hal/mmu_hal.h"
#include "hal/cache_ll.h"
#include "ff.h"
#include "driver/spi_master.h"
#include "driver/sdspi_host.h"
#include "driver/gpio.h"
#include "sdmmc_cmd.h"
#include "diskio_sdmmc.h"
#include "esp_rom_sys.h"

// app_main is provided by TinyGo object (kernel_idf.o).
extern void app_main(void);

// Per-domain RAM windows in internal DRAM.
// IDF linker places these in the .dram0.bss section at known fixed addresses.
// Addresses are read after build with xtensa-esp32s3-elf-nm and used as
// --defsym=__domain_ram_origin=0xADDR when linking each domain ELF.
#define DOMAIN_RAM_SLOT_SIZE 16384
#define DOMAIN_RAM_SLOT_SIZE_SDCARD 24576

__attribute__((aligned(4))) static uint8_t domain_ram_led[DOMAIN_RAM_SLOT_SIZE];
__attribute__((aligned(4))) static uint8_t domain_ram_wifi[DOMAIN_RAM_SLOT_SIZE];
__attribute__((aligned(4))) static uint8_t domain_ram_logger[DOMAIN_RAM_SLOT_SIZE];
__attribute__((aligned(4))) static uint8_t domain_ram_picoceci[DOMAIN_RAM_SLOT_SIZE];
__attribute__((aligned(4))) static uint8_t domain_ram_tls[DOMAIN_RAM_SLOT_SIZE];
__attribute__((aligned(4))) static uint8_t domain_ram_sdcard[DOMAIN_RAM_SLOT_SIZE_SDCARD];

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
    {
        *size_out = DOMAIN_RAM_SLOT_SIZE_SDCARD;
        return domain_ram_sdcard;
    }
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

// Initialize WiFi driver with ESP-IDF's canonical default config.
// This avoids relying on struct layout parity in TinyGo declarations.
int32_t canal_wifi_init_default(void)
{
    wifi_init_config_t cfg = WIFI_INIT_CONFIG_DEFAULT();
    return (int32_t)esp_wifi_init(&cfg);
}

// Allocate a persistent heap region for a domain from PSRAM.
// Returns NULL if PSRAM allocation is unavailable.
void *canal_domain_psram_alloc(uint32_t size)
{
    return heap_caps_malloc(size, MALLOC_CAP_SPIRAM | MALLOC_CAP_8BIT);
}

// SD card hardware initialisation via SPI.
// Initialises SPI2 bus, mounts the SDSPI host, probes the card, and
// registers the diskio driver so that FatFS f_mount() can succeed.
// Returns 0 on success, an esp_err_t code on failure.
#define SDCARD_SPI_HOST SPI2_HOST

static const char *TAG_SD = "canal_sd";

typedef struct
{
    int cs;
    int mosi;
    int miso;
    int clk;
} sd_pins_t;

// Canonical Canal ESP32-S3 SD SPI pins.
static const sd_pins_t k_sd_pin_sets[] = {
    //    {.cs = 4, .mosi = 11, .miso = 13, .clk = 12},
    //    {.cs = 2, .mosi = 42, .miso = 41, .clk = 40},
    {.cs = 13, .mosi = 15, .miso = 2, .clk = 14},
};

static sdmmc_card_t *s_sdcard = NULL;
static sdspi_dev_handle_t s_sdspi_handle = 0;
static bool s_bus_initialized = false;
static bool s_sdspi_initialized = false;
static int s_selected_pin_set = -1;

static void sd_spi_warmup_lines(const sd_pins_t pins)
{
    // Some adapters/cards need explicit clocks with CS high to enter SPI mode.
    gpio_set_direction(pins.cs, GPIO_MODE_OUTPUT);
    gpio_set_direction(pins.clk, GPIO_MODE_OUTPUT);
    gpio_set_direction(pins.mosi, GPIO_MODE_OUTPUT);
    gpio_set_direction(pins.miso, GPIO_MODE_INPUT);

    gpio_set_level(pins.cs, 1);
    gpio_set_level(pins.mosi, 1);
    gpio_set_level(pins.clk, 0);
    esp_rom_delay_us(200);

    for (int i = 0; i < 96; i++)
    {
        gpio_set_level(pins.clk, 1);
        esp_rom_delay_us(2);
        gpio_set_level(pins.clk, 0);
        esp_rom_delay_us(2);
    }

    esp_rom_delay_us(200);
}

int32_t canal_sdcard_init(void)
{
    if (s_sdcard != NULL)
    {
        return 0; // already initialised
    }

    static const int k_probe_freqs_khz[] = {SDMMC_FREQ_PROBING, 200, 100};
    esp_err_t last_err = ESP_FAIL;

    for (int i = 0; i < (int)(sizeof(k_sd_pin_sets) / sizeof(k_sd_pin_sets[0])); i++)
    {
        const sd_pins_t pins = k_sd_pin_sets[i];

        gpio_set_pull_mode(pins.mosi, GPIO_PULLUP_ONLY);
        gpio_set_pull_mode(pins.miso, GPIO_PULLUP_ONLY);
        gpio_set_pull_mode(pins.clk, GPIO_PULLUP_ONLY);
        gpio_set_pull_mode(pins.cs, GPIO_PULLUP_ONLY);
        sd_spi_warmup_lines(pins);
        vTaskDelay(pdMS_TO_TICKS(20));

        spi_bus_config_t bus_cfg = {
            .mosi_io_num = pins.mosi,
            .miso_io_num = pins.miso,
            .sclk_io_num = pins.clk,
            .quadwp_io_num = -1,
            .quadhd_io_num = -1,
            .max_transfer_sz = 4096,
        };
        esp_err_t ret = spi_bus_initialize(SDCARD_SPI_HOST, &bus_cfg, SDSPI_DEFAULT_DMA);
        if (ret == ESP_OK)
        {
            s_bus_initialized = true;
        }
        else if (ret != ESP_ERR_INVALID_STATE)
        {
            last_err = ret;
            ESP_LOGW(TAG_SD, "spi_bus_initialize failed pins[%d] cs=%d mosi=%d miso=%d clk=%d err=0x%x", i, pins.cs, pins.mosi, pins.miso, pins.clk, ret);
            continue;
        }

        sdspi_device_config_t slot_cfg = SDSPI_DEVICE_CONFIG_DEFAULT();
        slot_cfg.gpio_cs = pins.cs;
        slot_cfg.host_id = SDCARD_SPI_HOST;
        slot_cfg.gpio_cd = SDSPI_SLOT_NO_CD;
        slot_cfg.gpio_wp = SDSPI_SLOT_NO_WP;

        ret = sdspi_host_init();
        if (ret == ESP_OK)
        {
            s_sdspi_initialized = true;
        }
        else if (ret != ESP_ERR_INVALID_STATE)
        {
            last_err = ret;
            ESP_LOGW(TAG_SD, "sdspi_host_init failed pins[%d] err=0x%x", i, ret);
            if (s_bus_initialized)
            {
                spi_bus_free(SDCARD_SPI_HOST);
                s_bus_initialized = false;
            }
            continue;
        }

        ret = sdspi_host_init_device(&slot_cfg, &s_sdspi_handle);
        if (ret != ESP_OK)
        {
            last_err = ret;
            ESP_LOGW(TAG_SD, "sdspi_host_init_device failed pins[%d] err=0x%x", i, ret);
            if (s_sdspi_initialized)
            {
                sdspi_host_deinit();
                s_sdspi_initialized = false;
            }
            if (s_bus_initialized)
            {
                spi_bus_free(SDCARD_SPI_HOST);
                s_bus_initialized = false;
            }
            continue;
        }

        esp_err_t pinset_err = ESP_FAIL;
        for (int f = 0; f < (int)(sizeof(k_probe_freqs_khz) / sizeof(k_probe_freqs_khz[0])); f++)
        {
            sdmmc_host_t host = SDSPI_HOST_DEFAULT();
            host.slot = s_sdspi_handle;
            host.unaligned_multi_block_rw_max_chunk_size = 8;
            host.max_freq_khz = k_probe_freqs_khz[f];
            host.command_timeout_ms = 2000;
            ESP_LOGI(TAG_SD, "sd init try pins[%d] cs=%d mosi=%d miso=%d clk=%d freq=%dkHz", i, pins.cs, pins.mosi, pins.miso, pins.clk, host.max_freq_khz);
            vTaskDelay(pdMS_TO_TICKS(20));

            s_sdcard = heap_caps_malloc(sizeof(sdmmc_card_t), MALLOC_CAP_DEFAULT);
            if (!s_sdcard)
            {
                last_err = ESP_ERR_NO_MEM;
                pinset_err = ESP_ERR_NO_MEM;
                break;
            }

            ret = ESP_FAIL;
            for (int attempt = 0; attempt < 4; attempt++)
            {
                ret = sdmmc_card_init(&host, s_sdcard);
                if (ret == ESP_OK)
                {
                    break;
                }
                vTaskDelay(pdMS_TO_TICKS(120));
            }

            if (ret == ESP_OK)
            {
                ESP_LOGI(TAG_SD, "sd init ok, registering diskio. s_sdcard=%p", s_sdcard);
                ff_diskio_register_sdmmc(0, s_sdcard);
                s_selected_pin_set = i;
                ESP_LOGI(TAG_SD, "sd init ok pins[%d] cs=%d mosi=%d miso=%d clk=%d", i, pins.cs, pins.mosi, pins.miso, pins.clk);
                return 0;
            }

            pinset_err = ret;
            last_err = ret;
            ESP_LOGW(TAG_SD, "sdmmc_card_init failed pins[%d] freq=%dkHz err=0x%x", i, host.max_freq_khz, ret);
            heap_caps_free(s_sdcard);
            s_sdcard = NULL;
            sd_spi_warmup_lines(pins);
            vTaskDelay(pdMS_TO_TICKS(50));
        }

        sdspi_host_remove_device(s_sdspi_handle);
        s_sdspi_handle = 0;
        if (s_sdspi_initialized)
        {
            sdspi_host_deinit();
            s_sdspi_initialized = false;
        }
        if (s_bus_initialized)
        {
            spi_bus_free(SDCARD_SPI_HOST);
            s_bus_initialized = false;
        }

        if (pinset_err == ESP_ERR_NO_MEM)
        {
            return (int32_t)pinset_err;
        }
    }

    return (int32_t)last_err;
}

// FatFS wrappers exposed for TinyGo domains through --just-symbols.
uint8_t canal_f_mount(void *fs, const char *path, uint8_t opt)
{
    ESP_LOGI(TAG_SD, "canal_f_mount called: fs=%p path_ptr=%p opt=%u s_sdcard=%p", fs, path, opt, s_sdcard);
    uint8_t result = (uint8_t)f_mount((FATFS *)fs, path, opt);
    ESP_LOGI(TAG_SD, "canal_f_mount returned: %u", result);
    return result;
}

uint8_t canal_f_open(void *fp, const char *path, uint8_t mode)
{
    return (uint8_t)f_open((FIL *)fp, path, mode);
}

uint8_t canal_f_close(void *fp)
{
    return (uint8_t)f_close((FIL *)fp);
}

uint8_t canal_f_read(void *fp, void *buff, uint32_t btr, uint32_t *br)
{
    UINT out = 0;
    FRESULT res = f_read((FIL *)fp, buff, (UINT)btr, &out);
    if (br)
    {
        *br = (uint32_t)out;
    }
    return (uint8_t)res;
}

uint8_t canal_f_write(void *fp, const void *buff, uint32_t btw, uint32_t *bw)
{
    UINT out = 0;
    FRESULT res = f_write((FIL *)fp, buff, (UINT)btw, &out);
    if (bw)
    {
        *bw = (uint32_t)out;
    }
    return (uint8_t)res;
}

uint8_t canal_f_lseek(void *fp, uint32_t ofs)
{
    return (uint8_t)f_lseek((FIL *)fp, (FSIZE_t)ofs);
}

uint8_t canal_f_sync(void *fp)
{
    return (uint8_t)f_sync((FIL *)fp);
}

uint32_t canal_f_size(void *fp)
{
    return (uint32_t)f_size((FIL *)fp);
}

uint8_t canal_f_stat(const char *path, void *fno)
{
    return (uint8_t)f_stat(path, (FILINFO *)fno);
}

uint8_t canal_f_opendir(void *dp, const char *path)
{
    return (uint8_t)f_opendir((FF_DIR *)dp, path);
}

uint8_t canal_f_closedir(void *dp)
{
    return (uint8_t)f_closedir((FF_DIR *)dp);
}

uint8_t canal_f_readdir(void *dp, void *fno)
{
    return (uint8_t)f_readdir((FF_DIR *)dp, (FILINFO *)fno);
}

uint8_t canal_f_mkdir(const char *path)
{
    return (uint8_t)f_mkdir(path);
}

uint8_t canal_f_unlink(const char *path)
{
    return (uint8_t)f_unlink(path);
}

uint8_t canal_f_rename(const char *old_path, const char *new_path)
{
    return (uint8_t)f_rename(old_path, new_path);
}

uint8_t canal_f_truncate(void *fp)
{
    return (uint8_t)f_truncate((FIL *)fp);
}
