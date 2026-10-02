// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2022 The Ebitengine Authors

//go:build darwin || freebsd || linux || netbsd

#include "textflag.h"
#include "abi_amd64.h"
#include "go_asm.h"
#include "funcdata.h"

#define STACK_SIZE 224
#define PTR_ADDRESS (STACK_SIZE - 8)

// syscallX calls a function in libc on behalf of the syscall package.
// syscallX takes a pointer to a struct like:
// struct {
//	fn    uintptr
//	a1    uintptr
//	a2    uintptr
//	a3    uintptr
//	a4    uintptr
//	a5    uintptr
//	a6    uintptr
//	a7    uintptr
//	a8    uintptr
//	a9    uintptr
//	a10    uintptr
//	a11    uintptr
//	a12    uintptr
//	a13    uintptr
//	a14    uintptr
//	a15    uintptr
//	r1    uintptr
//	r2    uintptr
//	err   uintptr
// }
// syscallX must be called on the g0 stack with the
// C calling convention (use libcCall).
GLOBL ·syscallXABI0(SB), NOPTR|RODATA, $8
DATA ·syscallXABI0(SB)/8, $syscallX(SB)
TEXT syscallX(SB), NOSPLIT, $STACK_SIZE
	MOVQ  DI, PTR_ADDRESS(SP) // save the pointer
	MOVQ  DI, R11

	MOVQ syscallArgs_f1(R11), X0 // f1
	MOVQ syscallArgs_f2(R11), X1 // f2
	MOVQ syscallArgs_f3(R11), X2 // f3
	MOVQ syscallArgs_f4(R11), X3 // f4
	MOVQ syscallArgs_f5(R11), X4 // f5
	MOVQ syscallArgs_f6(R11), X5 // f6
	MOVQ syscallArgs_f7(R11), X6 // f7
	MOVQ syscallArgs_f8(R11), X7 // f8

	MOVQ syscallArgs_a1(R11), DI // a1
	MOVQ syscallArgs_a2(R11), SI // a2
	MOVQ syscallArgs_a3(R11), DX // a3
	MOVQ syscallArgs_a4(R11), CX // a4
	MOVQ syscallArgs_a5(R11), R8 // a5
	MOVQ syscallArgs_a6(R11), R9 // a6

	// push the remaining parameters onto the stack
	MOVQ syscallArgs_a7(R11), R12
	MOVQ R12, 0(SP)                  // push a7
	MOVQ syscallArgs_a8(R11), R12
	MOVQ R12, 8(SP)                  // push a8
	MOVQ syscallArgs_a9(R11), R12
	MOVQ R12, 16(SP)                 // push a9
	MOVQ syscallArgs_a10(R11), R12
	MOVQ R12, 24(SP)                 // push a10
	MOVQ syscallArgs_a11(R11), R12
	MOVQ R12, 32(SP)                 // push a11
	MOVQ syscallArgs_a12(R11), R12
	MOVQ R12, 40(SP)                 // push a12
	MOVQ syscallArgs_a13(R11), R12
	MOVQ R12, 48(SP)                 // push a13
	MOVQ syscallArgs_a14(R11), R12
	MOVQ R12, 56(SP)                 // push a14
	MOVQ syscallArgs_a15(R11), R12
	MOVQ R12, 64(SP)                 // push a15
	MOVQ syscallArgs_a16(R11), R12
	MOVQ R12, 72(SP)                 // push a16
	MOVQ syscallArgs_a17(R11), R12
	MOVQ R12, 80(SP)                 // push a17
	MOVQ syscallArgs_a18(R11), R12
	MOVQ R12, 88(SP)                 // push a18
	MOVQ syscallArgs_a19(R11), R12
	MOVQ R12, 96(SP)                 // push a19
	MOVQ syscallArgs_a20(R11), R12
	MOVQ R12, 104(SP)                // push a20
	MOVQ syscallArgs_a21(R11), R12
	MOVQ R12, 112(SP)                // push a21
	MOVQ syscallArgs_a22(R11), R12
	MOVQ R12, 120(SP)                // push a22
	MOVQ syscallArgs_a23(R11), R12
	MOVQ R12, 128(SP)                // push a23
	MOVQ syscallArgs_a24(R11), R12
	MOVQ R12, 136(SP)                // push a24
	MOVQ syscallArgs_a25(R11), R12
	MOVQ R12, 144(SP)                // push a25
	MOVQ syscallArgs_a26(R11), R12
	MOVQ R12, 152(SP)                // push a26
	MOVQ syscallArgs_a27(R11), R12
	MOVQ R12, 160(SP)                // push a27
	MOVQ syscallArgs_a28(R11), R12
	MOVQ R12, 168(SP)                // push a28
	MOVQ syscallArgs_a29(R11), R12
	MOVQ R12, 176(SP)                // push a29
	MOVQ syscallArgs_a30(R11), R12
	MOVQ R12, 184(SP)                // push a30
	MOVQ syscallArgs_a31(R11), R12
	MOVQ R12, 192(SP)                // push a31
	MOVQ syscallArgs_a32(R11), R12
	MOVQ R12, 200(SP)                // push a32
	XORL AX, AX                      // vararg: say "no float args"

	MOVQ syscallArgs_fn(R11), R10 // fn
	CALL R10

	MOVQ PTR_ADDRESS(SP), DI      // get the pointer back
	MOVQ AX, syscallArgs_a1(DI) // r1
	MOVQ DX, syscallArgs_a2(DI) // r2
	MOVQ X0, syscallArgs_f1(DI) // f1
	MOVQ X1, syscallArgs_f2(DI) // f2

#ifdef GOOS_darwin
	CALL purego_error(SB)
	MOVQ PTR_ADDRESS(SP), DI      // reload (DI clobbered by call)
	MOVQ (AX), AX
	MOVQ AX, syscallArgs_a3(DI) // save errno
#else
	MOVQ $0, syscallArgs_a3(DI) // no errno here: clear the input argument
#endif

	XORL AX, AX          // no error (it's ignored anyway)
	RET

TEXT callbackasm1(SB), NOSPLIT|NOFRAME, $0
	MOVQ 0(SP), AX  // save the return address to calculate the cb index
	MOVQ 8(SP), R10 // get the return SP so that we can align register args with stack args
	ADDQ $8, SP     // remove return address from stack, we are not returning to callbackasm, but to its caller.

	// make space for first six int and 8 float arguments below the frame
	ADJSP $14*8, SP
	MOVSD X0, (1*8)(SP)
	MOVSD X1, (2*8)(SP)
	MOVSD X2, (3*8)(SP)
	MOVSD X3, (4*8)(SP)
	MOVSD X4, (5*8)(SP)
	MOVSD X5, (6*8)(SP)
	MOVSD X6, (7*8)(SP)
	MOVSD X7, (8*8)(SP)
	MOVQ  DI, (9*8)(SP)
	MOVQ  SI, (10*8)(SP)
	MOVQ  DX, (11*8)(SP)
	MOVQ  CX, (12*8)(SP)
	MOVQ  R8, (13*8)(SP)
	MOVQ  R9, (14*8)(SP)
	LEAQ  8(SP), R8      // R8 = address of args vector

	PUSHQ R10 // push the stack pointer below registers

	// Switch from the host ABI to the Go ABI.
	PUSH_REGS_HOST_TO_ABI0()

	// determine index into runtime·cbs table
	MOVQ $callbackasm(SB), DX
	SUBQ DX, AX
	MOVQ $0, DX
	MOVQ $5, CX               // divide by 5 because each call instruction in ·callbacks is 5 bytes long
	DIVL CX
	SUBQ $1, AX               // subtract 1 because return PC is to the next slot

	// Create a struct callbackArgs on our stack to be passed as
	// the "frame" to cgocallback and on to callbackWrap.
	// $24 to make enough room for the arguments to runtime.cgocallback
	SUBQ $(24+callbackArgs__size), SP
	MOVQ AX, (24+callbackArgs_index)(SP)  // callback index
	MOVQ R8, (24+callbackArgs_args)(SP)   // address of args vector
	MOVQ DI, (24+callbackArgs_result)(SP) // result
	LEAQ 24(SP), AX                       // take the address of callbackArgs

	// Call cgocallback, which will call callbackWrap(frame).
	MOVQ ·callbackWrap_call(SB), DI // Get the ABIInternal function pointer
	MOVQ (DI), DI                   // without <ABIInternal> by using a closure.
	MOVQ AX, SI                     // frame (address of callbackArgs)
	MOVQ $0, CX                     // context

	CALL crosscall2(SB) // runtime.cgocallback(fn, frame, ctxt uintptr)

	// Get callback result.
	MOVQ (24+callbackArgs_result)(SP), AX
	MOVQ (24+callbackArgs_result+8)(SP), DX
	// Restore XMM0/XMM1 from result[2]/result[3] for struct eightbytes
	// classified as SSE by the SysV ABI.
	MOVQ (24+callbackArgs_result+16)(SP), X0
	MOVQ (24+callbackArgs_result+24)(SP), X1
	ADDQ $(24+callbackArgs__size), SP      // remove callbackArgs struct

	POP_REGS_HOST_TO_ABI0()

	POPQ  R10        // get the SP back
	ADJSP $-14*8, SP // remove arguments

	MOVQ R10, 0(SP)

	RET
