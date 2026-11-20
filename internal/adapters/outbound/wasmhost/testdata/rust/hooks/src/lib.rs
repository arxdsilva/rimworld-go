// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// All six hooks. Each one reads the event and every read import, then sends
// a message echoing what it read, a letter echoing the texts, and spawns 2
// wood, so the host tests can assert each value.
#![no_std]
#![allow(dead_code)]

const BUF_SIZE: usize = 4096;
include!("../../prelude.rs");

static mut RECS: [u8; 256] = [0; 256]; // 8 records
static mut SEA: [u8; 32] = [0; 32];
static mut NAME: [u8; 32] = [0; 32];
static mut ONE: [u8; 32] = [0; 32];

const WOOD: &str = "core.wood";

fn echo(name: &str) {
    let tick = rd64(8);
    let nrec = rd32(16) as usize;
    let l1 = rd32(20) as usize;
    let l2 = rd32(24) as usize;
    let t1 = 32 + nrec * 32;
    let t2 = t1 + l1;

    let mut w = W::new();
    w.s(name);
    w.s(" tick=");
    w.u(tick);
    w.s(" nrec=");
    w.u(nrec as u64);
    w.s(" l1=");
    w.u(l1 as u64);
    w.s(" l2=");
    w.u(l2 as u64);
    unsafe {
        w.s(" day=");
        w.i(day() as i64);
        w.s(" hour=");
        w.i(hour() as i64);
        w.s(" col=");
        w.i(colonists() as i64);
        w.s(" wealth=");
        w.i(wealth());
        w.s(" wood=");
        w.i(count(WOOD.as_ptr() as i32, WOOD.len() as i32) as i64);
        w.s(" rand=");
        w.i(rand(1000) as i64);
        let sl = season(SEA.as_ptr() as i32, 32);
        w.s(" season=");
        w.i(sl as i64);
        w.s(":");
        let sn = if sl < 32 { sl } else { 32 } as usize;
        w.raw(SEA.as_ptr(), sn);
        let total = pawns(RECS.as_ptr() as i32, 8);
        let first = core::ptr::read_unaligned(RECS.as_ptr() as *const i64);
        w.s(" pawns=");
        w.i(total as i64);
        w.s(" first=");
        w.i(first);
        let got = pawn(first, ONE.as_ptr() as i32);
        w.s(" pawn=");
        w.i(got as i64);
        w.s(" food=");
        w.i(core::ptr::read_unaligned(ONE.as_ptr().add(16) as *const i32) as i64);
        let nl = pawn_name(first, NAME.as_ptr() as i32, 32);
        w.s(" nl=");
        w.i(nl as i64);
        w.s(":");
        let nn = if nl < 0 { 0 } else if nl < 32 { nl } else { 32 } as usize;
        w.raw(NAME.as_ptr(), nn);
        w.s(" miss=");
        w.i(pawn(-1, ONE.as_ptr() as i32) as i64);
        w.s(",");
        w.i(pawn_name(-1, NAME.as_ptr() as i32, 32) as i64);
    }
    w.send();

    let mut l = W::new();
    l.s("t1=");
    l.raw(unsafe { buf().add(t1) }, l1);
    l.s(" t2=");
    l.raw(unsafe { buf().add(t2) }, l2);
    unsafe {
        letter(name.as_ptr() as i32, name.len() as i32, OUT.as_ptr() as i32, l.n as i32);
        spawn(WOOD.as_ptr() as i32, WOOD.len() as i32, 2);
    }
}

#[no_mangle]
pub extern "C" fn on_hour() {
    echo("on_hour");
}

#[no_mangle]
pub extern "C" fn on_day() {
    echo("on_day");
}

#[no_mangle]
pub extern "C" fn on_incident() {
    echo("on_incident");
}

#[no_mangle]
pub extern "C" fn on_colonist_died() {
    echo("on_colonist_died");
}

#[no_mangle]
pub extern "C" fn on_pawns() {
    echo("on_pawns");
}

#[no_mangle]
pub extern "C" fn on_pawn() {
    echo("on_pawn");
}
