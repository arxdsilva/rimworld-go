// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// A counter kept in guest memory: on_hour increments it and sends
// "count N". save/load serialise it as 4 little-endian bytes.
#![no_std]
#![allow(dead_code)]

const BUF_SIZE: usize = 256;
include!("../../prelude.rs");

static mut COUNT: u32 = 0;

#[no_mangle]
pub extern "C" fn on_hour() {
    let c = unsafe {
        COUNT += 1;
        COUNT
    };
    let mut w = W::new();
    w.s("count ");
    w.u(c as u64);
    w.send();
}

#[no_mangle]
pub extern "C" fn state_version() -> i32 {
    1
}

#[no_mangle]
pub extern "C" fn save() -> i32 {
    unsafe { core::ptr::write_unaligned(buf() as *mut u32, COUNT) };
    4
}

#[no_mangle]
pub extern "C" fn load(len: i32) -> i32 {
    if len != 4 {
        return 1;
    }
    unsafe { COUNT = rd32(0) };
    0
}
