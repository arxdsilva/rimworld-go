// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// scriptbench workload, per call: colonists(), then pawn(id) for id in
// 1..=N, count food < 30, sum mood, message "%d %d".
#![no_std]
#![allow(dead_code)]

const BUF_SIZE: usize = 256;
include!("../../prelude.rs");

#[no_mangle]
pub extern "C" fn on_hour() {
    let n = unsafe { colonists() };
    let mut low = 0i64;
    let mut mood = 0i64;
    for id in 1..=n {
        if unsafe { pawn(id as i64, rw_buf()) } == 1 {
            if rdi32(16) < 30 {
                low += 1;
            }
            mood += rdi32(24) as i64;
        }
    }
    let mut w = W::new();
    w.i(low);
    w.s(" ");
    w.i(mood);
    w.send();
}
