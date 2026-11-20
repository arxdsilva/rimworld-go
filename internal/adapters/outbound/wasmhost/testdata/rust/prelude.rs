// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// Shared by every fixture crate: `include!("../../prelude.rs")` from
// src/lib.rs. ABI v1 of wasmhost/doc.go. Written for rustc 1.39.0.

use core::arch::wasm32;
use core::panic::PanicInfo;

#[panic_handler]
fn panic(_: &PanicInfo) -> ! {
    unsafe { wasm32::unreachable() }
}

// The event buffer (rw_buf). A crate may set BUF_SIZE before including.
static mut BUF: [u8; BUF_SIZE] = [0; BUF_SIZE];

#[no_mangle]
pub extern "C" fn rw_buf() -> i32 {
    unsafe { BUF.as_ptr() as i32 }
}

#[no_mangle]
pub extern "C" fn rw_buf_size() -> i32 {
    BUF_SIZE as i32
}

#[link(wasm_import_module = "rimworld")]
extern "C" {
    fn message(p: i32, n: i32);
    fn letter(tp: i32, tn: i32, p: i32, n: i32);
    fn spawn(kp: i32, kn: i32, count: i32);
    fn rand(n: i32) -> i32;
    fn count(kp: i32, kn: i32) -> i32;
    fn day() -> i32;
    fn hour() -> i32;
    fn colonists() -> i32;
    fn wealth() -> i64;
    fn season(p: i32, cap: i32) -> i32;
    fn pawns(p: i32, cap: i32) -> i32;
    fn pawn(id: i64, p: i32) -> i32;
    fn pawn_name(id: i64, p: i32, cap: i32) -> i32;
}

fn buf() -> *mut u8 {
    unsafe { BUF.as_mut_ptr() }
}

fn rd32(off: usize) -> u32 {
    unsafe { core::ptr::read_unaligned(buf().add(off) as *const u32) }
}

fn rd64(off: usize) -> u64 {
    unsafe { core::ptr::read_unaligned(buf().add(off) as *const u64) }
}

fn rdi32(off: usize) -> i32 {
    rd32(off) as i32
}

// Text builder over a separate static, so the event in BUF stays readable.
const OUT_SIZE: usize = 512;
static mut OUT: [u8; OUT_SIZE] = [0; OUT_SIZE];

struct W {
    n: usize,
}

impl W {
    fn new() -> W {
        W { n: 0 }
    }

    fn b(&mut self, c: u8) {
        if self.n < OUT_SIZE {
            unsafe { OUT[self.n] = c };
            self.n += 1;
        }
    }

    fn s(&mut self, s: &str) {
        for c in s.bytes() {
            self.b(c);
        }
    }

    fn raw(&mut self, p: *const u8, n: usize) {
        for i in 0..n {
            let c = unsafe { *p.add(i) };
            self.b(c);
        }
    }

    // decimal, like Starlark's %d
    fn i(&mut self, v: i64) {
        if v < 0 {
            self.b(b'-');
            self.u((v as u64).wrapping_neg());
        } else {
            self.u(v as u64);
        }
    }

    fn u(&mut self, mut v: u64) {
        let mut t = [0u8; 20];
        let mut k = 0;
        loop {
            t[k] = b'0' + (v % 10) as u8;
            k += 1;
            v /= 10;
            if v == 0 {
                break;
            }
        }
        while k > 0 {
            k -= 1;
            self.b(t[k]);
        }
    }

    fn x(&mut self, v: u64) {
        for sh in (0..16u32).rev() {
            let d = ((v >> (sh * 4)) & 15) as u8;
            self.b(if d < 10 { b'0' + d } else { b'a' + d - 10 });
        }
    }

    fn send(&self) {
        unsafe { message(OUT.as_ptr() as i32, self.n as i32) }
    }
}
