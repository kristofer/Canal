/* Canal domain linker script for ESP32-S3
 *
 * Each domain binary is a self-contained ELF flashed to its own partition.
 * The kernel's ELF loader reads program headers, copies LOAD segments into
 * the domain's RAM region, and jumps to domain_entry.
 *
 * Flash XIP mapping: physical 0xN0000 → virtual 0x420N0000
 * The DOMAIN_FLASH origin below must match the domain's partition offset
 * after adding the XIP base (0x42000000).
 *
 * Usage with TinyGo (pass via -ldflags):
 *   tinygo build -ldflags="-T build/targets/esp32s3/domain-linker.ld" ...
 *
 * If TinyGo ignores the external script, use -ldflags="-e domain_entry"
 * to at least set the ELF entry point to domain_entry.
 */

ENTRY(domain_entry)

/* Per-domain overrides passed via --defsym:
 *   __domain_ram_origin=0xADDR   (address of domain_ram_<name> in bridge_main.c)
 *   __domain_ram_length=0x1000
 */
__domain_ram_origin = DEFINED(__domain_ram_origin) ? __domain_ram_origin : 0x3FC90000;
__domain_ram_length = DEFINED(__domain_ram_length) ? __domain_ram_length : 0x1000;

MEMORY
{
    /* Domain code lives in its flash partition (XIP).
     * Loader sets up the correct virtual address per partition. */
    DOMAIN_FLASH (rx)  : ORIGIN = 0x42100000, LENGTH = 64K

    /* Per-domain SRAM region — kernel allocates and passes base at spawn time */
    DOMAIN_RAM   (rw)  : ORIGIN = __domain_ram_origin, LENGTH = __domain_ram_length
}

SECTIONS
{
    .text : {
        KEEP(*(.text.domain_entry))   /* entry point first */
        *(.text*)
        *(.rodata*)
    } > DOMAIN_FLASH

    .data : {
        _data_start = .;
        *(.data*)
        _data_end = .;
    } > DOMAIN_RAM AT > DOMAIN_FLASH

    .bss (NOLOAD) : {
        _bss_start = .;
        *(.bss*)
        *(COMMON)
        _bss_end = .;
    } > DOMAIN_RAM

    /* Discard debug and other non-essential sections to save flash */
    /DISCARD/ : {
        *(.debug*)
        *(.comment)
        *(.eh_frame*)
    }
}
