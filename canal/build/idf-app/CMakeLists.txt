cmake_minimum_required(VERSION 3.16)

set(IDF_TARGET esp32s3)

include($ENV{IDF_PATH}/tools/cmake/project.cmake)
project(canal_idf_bridge)

# TinyGo runtime expects legacy bss boundary symbols.
idf_build_get_property(PROJECT_EXECUTABLE EXECUTABLE)
target_link_options(${PROJECT_EXECUTABLE} PRIVATE
	"-Wl,--defsym=_sbss=_bss_start"
	"-Wl,--defsym=_ebss=_bss_end"
	"-Wl,-u,usb_serial_jtag_vfs_use_nonblocking"
	"-Wl,-u,usb_serial_jtag_driver_install"
	"-Wl,-u,usb_serial_jtag_vfs_use_driver"
	"-Wl,-u,usb_serial_jtag_is_driver_installed"
	"-Wl,-u,usb_serial_jtag_write_bytes"
	"-Wl,-u,usb_serial_jtag_wait_tx_done"
	"-Wl,-u,canal_domain_psram_alloc"
	"-Wl,-u,esp_wifi_init"
	"-Wl,-u,esp_wifi_set_mode"
	"-Wl,-u,esp_wifi_set_config"
	"-Wl,-u,esp_wifi_start"
	"-Wl,-u,esp_wifi_connect"
	"-Wl,-u,lwip_socket"
	"-Wl,-u,lwip_bind"
	"-Wl,-u,lwip_listen"
	"-Wl,-u,lwip_accept"
	"-Wl,-u,lwip_recv"
	"-Wl,-u,lwip_send"
	"-Wl,-u,lwip_close"
	"-Wl,-u,lwip_setsockopt"
	"-Wl,-u,lwip_fcntl"
	"-Wl,-u,esp_netif_init"
	"-Wl,-u,esp_netif_get_ip_info"
	"-Wl,-u,esp_event_loop_create_default"
	"-Wl,-u,esp_netif_create_default_wifi_sta"
	"-Wl,-u,nvs_flash_init"
	"-Wl,-u,nvs_flash_erase"
)
