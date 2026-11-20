// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// scriptbench workload, batched: one pawns() call, count food < 30, sum
// mood, message "%d %d". Records land at offset 0 of the buffer (room for
// 512 colonists).
#![no_std]
#![allow(dead_code)]

const BUF_SIZE: usize = 16384;
include!("../../prelude.rs");

#[no_mangle]
pub extern "C" fn on_hour() {
    let cap = (BUF_SIZE / 32) as i32;
    let total = unsafe { pawns(rw_buf(), cap) };
    let n = if total < cap { total } else { cap } as usize;
    let mut low = 0i64;
    let mut mood = 0i64;
    for i in 0..n {
        let r = i * 32;
        if rdi32(r + 16) < 30 {
            low += 1;
        }
        mood += rdi32(r + 24) as i64;
    }
    let mut w = W::new();
    w.i(low);
    w.s(" ");
    w.i(mood);
    w.send();
}
