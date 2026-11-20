// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// A deterministic mix over the events: integer and f64/f32 arithmetic on
// host-supplied values, including x/x with x = 0, inf - inf and 0 * inf, and
// the reinterpreted bits in the message. No sqrt (std-only in 1.39).
#![no_std]
#![allow(dead_code)]

const BUF_SIZE: usize = 4096;
include!("../../prelude.rs");

fn mix(name: &str) {
    let hook = rd32(4);
    let tick = rd64(8);
    let nrec = rd32(16) as usize;
    let l1 = rd32(20);

    // integer mix over every record the event carries
    let mut acc: u64 = 0xcbf29ce484222325 ^ tick;
    let mut fsum: f64 = 0.0;
    let mut i = 0;
    while i < nrec && 32 + (i + 1) * 32 <= BUF_SIZE {
        let r = 32 + i * 32;
        let id = rd64(r);
        acc = (acc ^ id).wrapping_mul(0x100000001b3);
        acc = (acc ^ (rdi32(r + 8) as i64 as u64)).wrapping_mul(0x100000001b3);
        acc = (acc ^ (rdi32(r + 12) as i64 as u64)).wrapping_mul(0x100000001b3);
        acc = (acc ^ (rdi32(r + 16) as i64 as u64)).wrapping_mul(0x100000001b3);
        acc = (acc ^ (rdi32(r + 24) as i64 as u64)).wrapping_mul(0x100000001b3);
        acc = (acc ^ rd32(r + 28) as u64).wrapping_mul(0x100000001b3);
        fsum += rdi32(r + 16) as f64 * 0.5 + rdi32(r + 24) as f64 / 3.0;
        i += 1;
    }

    // f64 values from the host: x/x with x = 0 (len1 is 0 for most hooks),
    // inf - inf, 0 * inf, and plain arithmetic
    let x = l1 as f64;
    let nan1 = x / x;
    let big = 1.0e300 * (1.0 + nrec as f64);
    let inf = big * big * big;
    let nan2 = inf - inf;
    let zero = nrec as f64 - nrec as f64;
    let nan3 = zero * inf;
    let plain = (fsum + 1.5) * 2.0 - tick as f64 / 7.0;
    let f32v = plain as f32 + nan1 as f32;

    let mut w = W::new();
    w.s(name);
    w.s(" hook=");
    w.u(hook as u64);
    w.s(" acc=");
    w.x(acc);
    w.s(" 0/0=");
    w.x(nan1.to_bits());
    w.s(" inf-inf=");
    w.x(nan2.to_bits());
    w.s(" 0*inf=");
    w.x(nan3.to_bits());
    w.s(" plain=");
    w.x(plain.to_bits());
    w.s(" f32=");
    w.x(f32v.to_bits() as u64);
    w.send();
}

#[no_mangle]
pub extern "C" fn on_hour() {
    mix("on_hour");
}

#[no_mangle]
pub extern "C" fn on_day() {
    mix("on_day");
}

#[no_mangle]
pub extern "C" fn on_incident() {
    mix("on_incident");
}

#[no_mangle]
pub extern "C" fn on_colonist_died() {
    mix("on_colonist_died");
}

#[no_mangle]
pub extern "C" fn on_pawns() {
    mix("on_pawns");
}

#[no_mangle]
pub extern "C" fn on_pawn() {
    mix("on_pawn");
}
