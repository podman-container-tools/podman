// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The Ebitengine Authors

//go:build linux

#include "textflag.h"
#include "go_asm.h"
#include "funcdata.h"

#define STACK_SIZE 128
#define PTR_ADDRESS (STACK_SIZE - 4)

// syscallX calls a function in libc on behalf of the syscall package.
// syscallX takes a pointer to a struct like:
// struct {
//	fn    uintptr
//	a1    uintptr
//  ...
//	a32   uintptr
//	f1    uintptr
//	...
//	f16   uintptr
//	arm64_r8 uintptr
// }
// syscallX must be called on the g0 stack with the
// C calling convention (use libcCall).
GLOBL ·syscallXABI0(SB), NOPTR|RODATA, $4
DATA ·syscallXABI0(SB)/4, $syscallX(SB)
TEXT syscallX(SB), NOSPLIT|NOFRAME, $0-0
	// Called via C calling convention: R0 = pointer to syscallArgs
	// NOT via Go calling convention
	// Save link register and callee-saved registers first
	MOVW.W    R14, -4(R13)                         // save LR (decrement and store)
	MOVM.DB.W [R4, R5, R6, R7, R8, R9, R11], (R13) // save callee-saved regs

	MOVW R0, R8
	SUB  $STACK_SIZE, R13
	MOVW R8, PTR_ADDRESS(R13)

	// Load function pointer first (before anything can corrupt R8)
	MOVW syscallArgs_fn(R8), R5
	MOVW R5, (PTR_ADDRESS-4)(R13) // save fn at offset 56

	// Skip floating point registers if runtime.goarmsoftfp!=0.
	MOVB    runtime·goarmsoftfp(SB), R11
	CMP     $0, R11
	BNE     skipfpload
	// Load floating point arguments
	// Each float64 spans 2 uintptr slots (8 bytes) on ARM32, so we skip by 2
	MOVD syscallArgs_f1(R8), F0  // f1+f2 -> D0
	MOVD syscallArgs_f3(R8), F1  // f3+f4 -> D1
	MOVD syscallArgs_f5(R8), F2  // f5+f6 -> D2
	MOVD syscallArgs_f7(R8), F3  // f7+f8 -> D3
	MOVD syscallArgs_f9(R8), F4  // f9+f10 -> D4
	MOVD syscallArgs_f11(R8), F5 // f11+f12 -> D5
	MOVD syscallArgs_f13(R8), F6 // f13+f14 -> D6
	MOVD syscallArgs_f15(R8), F7 // f15+f16 -> D7

skipfpload:
	// Load integer arguments into registers (R0-R3 for ARM EABI)
	MOVW syscallArgs_a1(R8), R0 // a1
	MOVW syscallArgs_a2(R8), R1 // a2
	MOVW syscallArgs_a3(R8), R2 // a3
	MOVW syscallArgs_a4(R8), R3 // a4

	// push a5-a32 onto stack
	MOVW syscallArgs_a5(R8), R4
	MOVW R4, 0(R13)
	MOVW syscallArgs_a6(R8), R4
	MOVW R4, 4(R13)
	MOVW syscallArgs_a7(R8), R4
	MOVW R4, 8(R13)
	MOVW syscallArgs_a8(R8), R4
	MOVW R4, 12(R13)
	MOVW syscallArgs_a9(R8), R4
	MOVW R4, 16(R13)
	MOVW syscallArgs_a10(R8), R4
	MOVW R4, 20(R13)
	MOVW syscallArgs_a11(R8), R4
	MOVW R4, 24(R13)
	MOVW syscallArgs_a12(R8), R4
	MOVW R4, 28(R13)
	MOVW syscallArgs_a13(R8), R4
	MOVW R4, 32(R13)
	MOVW syscallArgs_a14(R8), R4
	MOVW R4, 36(R13)
	MOVW syscallArgs_a15(R8), R4
	MOVW R4, 40(R13)
	MOVW syscallArgs_a16(R8), R4
	MOVW R4, 44(R13)
	MOVW syscallArgs_a17(R8), R4
	MOVW R4, 48(R13)
	MOVW syscallArgs_a18(R8), R4
	MOVW R4, 52(R13)
	MOVW syscallArgs_a19(R8), R4
	MOVW R4, 56(R13)
	MOVW syscallArgs_a20(R8), R4
	MOVW R4, 60(R13)
	MOVW syscallArgs_a21(R8), R4
	MOVW R4, 64(R13)
	MOVW syscallArgs_a22(R8), R4
	MOVW R4, 68(R13)
	MOVW syscallArgs_a23(R8), R4
	MOVW R4, 72(R13)
	MOVW syscallArgs_a24(R8), R4
	MOVW R4, 76(R13)
	MOVW syscallArgs_a25(R8), R4
	MOVW R4, 80(R13)
	MOVW syscallArgs_a26(R8), R4
	MOVW R4, 84(R13)
	MOVW syscallArgs_a27(R8), R4
	MOVW R4, 88(R13)
	MOVW syscallArgs_a28(R8), R4
	MOVW R4, 92(R13)
	MOVW syscallArgs_a29(R8), R4
	MOVW R4, 96(R13)
	MOVW syscallArgs_a30(R8), R4
	MOVW R4, 100(R13)
	MOVW syscallArgs_a31(R8), R4
	MOVW R4, 104(R13)
	MOVW syscallArgs_a32(R8), R4
	MOVW R4, 108(R13)

	// Load saved function pointer and call
	MOVW (PTR_ADDRESS-4)(R13), R4

	// Use BLX for Thumb interworking - Go assembler doesn't support BLX Rn
	// BLX R4 = 0xE12FFF34 (ARM encoding, always condition)
	WORD $0xE12FFF34 // blx r4

	// pop structure pointer
	MOVW PTR_ADDRESS(R13), R8
	ADD  $STACK_SIZE, R13

	// save R0, R1
	MOVW R0, syscallArgs_a1(R8)
	MOVW R1, syscallArgs_a2(R8)
	MOVW $0, R1
	MOVW R1, syscallArgs_a3(R8) // no errno here: clear the input argument

	MOVB    runtime·goarmsoftfp(SB), R11
	CMP     $0, R11
	BNE     skipfpsave
	// save f0-f3 (each float64 spans 2 uintptr slots on ARM32)
	MOVD F0, syscallArgs_f1(R8)
	MOVD F1, syscallArgs_f3(R8)
	MOVD F2, syscallArgs_f5(R8)
	MOVD F3, syscallArgs_f7(R8)

skipfpsave:
	// Restore callee-saved registers and return
	MOVM.IA.W (R13), [R4, R5, R6, R7, R8, R9, R11]
	MOVW.P    4(R13), R15                          // pop LR into PC (return)
