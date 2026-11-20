// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// The randSpawner and pawnSpawner scripts of internal/compose/game/
// scripts_test.go, one to one. The text matches Starlark's %d formatting.
#![no_std]
#![allow(dead_code)]

const BUF_SIZE: usize = 256;
include!("../../prelude.rs");

const WOOD: &str = "core.wood";

fn drop_wood(n: i32) {
    unsafe { spawn(WOOD.as_ptr() as i32, WOOD.len() as i32, n) };
}

#[no_mangle]
pub extern "C" fn on_hour() {
    let n = 1 + unsafe { rand(9) };
    drop_wood(n);
    let mut w = W::new();
    w.s("dropped ");
    w.i(n as i64);
    w.s(" wood at ");
    w.i(unsafe { hour() } as i64);
    w.s(":00");
    w.send();
}

#[no_mangle]
pub extern "C" fn on_pawn() {
    let id = rd64(32);
    if unsafe { rand(4) } == 0 {
        drop_wood(1 + (id % 3) as i32);
    }
}
