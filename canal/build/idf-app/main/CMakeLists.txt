idf_component_register(SRCS "bridge_main.c"
                    INCLUDE_DIRS ".")

set(CANAL_ROOT "${CMAKE_CURRENT_LIST_DIR}/../../..")
set(CANAL_KERNEL_DIR "${CANAL_ROOT}/kernel")
set(CANAL_OBJ "${CMAKE_BINARY_DIR}/kernel_idf.o")
set(CANAL_OBJ_RAW "${CMAKE_BINARY_DIR}/kernel_idf_raw.o")

set(TINYGO_REDEFINE_ARGS
    --redefine-sym malloc=tinygo_malloc
    --redefine-sym calloc=tinygo_calloc
    --redefine-sym free=tinygo_free
    --redefine-sym __atomic_add_fetch_2=tinygo___atomic_add_fetch_2
    --redefine-sym __atomic_add_fetch_4=tinygo___atomic_add_fetch_4
    --redefine-sym __atomic_add_fetch_8=tinygo___atomic_add_fetch_8
    --redefine-sym __atomic_compare_exchange_2=tinygo___atomic_compare_exchange_2
    --redefine-sym __atomic_compare_exchange_4=tinygo___atomic_compare_exchange_4
    --redefine-sym __atomic_compare_exchange_8=tinygo___atomic_compare_exchange_8
    --redefine-sym __atomic_exchange_2=tinygo___atomic_exchange_2
    --redefine-sym __atomic_exchange_4=tinygo___atomic_exchange_4
    --redefine-sym __atomic_exchange_8=tinygo___atomic_exchange_8
    --redefine-sym __atomic_fetch_add_2=tinygo___atomic_fetch_add_2
    --redefine-sym __atomic_fetch_add_4=tinygo___atomic_fetch_add_4
    --redefine-sym __atomic_fetch_add_8=tinygo___atomic_fetch_add_8
    --redefine-sym __atomic_load_2=tinygo___atomic_load_2
    --redefine-sym __atomic_load_4=tinygo___atomic_load_4
    --redefine-sym __atomic_load_8=tinygo___atomic_load_8
    --redefine-sym __atomic_store_2=tinygo___atomic_store_2
    --redefine-sym __atomic_store_4=tinygo___atomic_store_4
    --redefine-sym __atomic_store_8=tinygo___atomic_store_8
    --redefine-sym __sync_fetch_and_add_2=tinygo___sync_fetch_and_add_2
    --redefine-sym __sync_fetch_and_add_4=tinygo___sync_fetch_and_add_4
    --redefine-sym __sync_fetch_and_add_8=tinygo___sync_fetch_and_add_8
    --redefine-sym __sync_val_compare_and_swap_2=tinygo___sync_val_compare_and_swap_2
    --redefine-sym __sync_val_compare_and_swap_4=tinygo___sync_val_compare_and_swap_4
    --redefine-sym __sync_val_compare_and_swap_8=tinygo___sync_val_compare_and_swap_8
    --redefine-sym __sync_lock_test_and_set_2=tinygo___sync_lock_test_and_set_2
    --redefine-sym __sync_lock_test_and_set_4=tinygo___sync_lock_test_and_set_4
    --redefine-sym __sync_lock_test_and_set_8=tinygo___sync_lock_test_and_set_8
)

add_custom_command(
    OUTPUT ${CANAL_OBJ}
    COMMAND tinygo build -target=esp32s3-generic -tags=idf -opt=2 -scheduler=none -gc=leaking -o ${CANAL_OBJ_RAW} ./cmd/esp32s3
    COMMAND ${CMAKE_OBJCOPY} ${TINYGO_REDEFINE_ARGS} ${CANAL_OBJ_RAW} ${CANAL_OBJ}
    WORKING_DIRECTORY ${CANAL_KERNEL_DIR}
    COMMENT "Building TinyGo kernel object for IDF link"
    VERBATIM
)

add_custom_target(canal_tinygo_obj ALL DEPENDS ${CANAL_OBJ})
add_dependencies(${COMPONENT_LIB} canal_tinygo_obj)

target_link_libraries(${COMPONENT_LIB} INTERFACE ${CANAL_OBJ})
