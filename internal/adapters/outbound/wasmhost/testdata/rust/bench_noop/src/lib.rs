// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

// An empty on_hour: the cost of one hook call.
#![no_std]
#![allow(dead_code)]

const BUF_SIZE: usize = 64;
include!("../../prelude.rs");

#[no_mangle]
pub extern "C" fn on_hour() {}
