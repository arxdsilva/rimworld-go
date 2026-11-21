// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// A stateful spawner (PLG-063 AC-2, AC-4, AC-5): on_hour adds 1 to a
// counter and, every 2nd hour, spawns core.wood x counter (wrapped into
// 1..=500, the host's spawn cap, so a long run never traps). The colony
// depends on the counter, so a save without a restore changes the game.
// save/load serialise it as 4 little-endian bytes.
#![no_std]
#![allow(dead_code)]

const BUF_SIZE: usize = 256;
include!("../../prelude.rs");

const WOOD: &str = "core.wood";
const MAX_SPAWN: u32 = 500; // scripting.MaxSpawn

static mut TALLY: u32 = 0;

#[no_mangle]
pub extern "C" fn on_hour() {
    let c = unsafe {
        TALLY += 1;
        TALLY
    };
    if c % 2 == 0 {
        let n = (c - 1) % MAX_SPAWN + 1;
        unsafe { spawn(WOOD.as_ptr() as i32, WOOD.len() as i32, n as i32) };
    }
}

#[no_mangle]
pub extern "C" fn state_version() -> i32 {
    1
}

#[no_mangle]
pub extern "C" fn save() -> i32 {
    unsafe { core::ptr::write_unaligned(buf() as *mut u32, TALLY) };
    4
}

#[no_mangle]
pub extern "C" fn load(len: i32) -> i32 {
    if len != 4 {
        return 1;
    }
    unsafe { TALLY = rd32(0) };
    0
}
