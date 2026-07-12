// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The Ebitengine Authors

//go:build linux

#include "textflag.h"
#include "go_asm.h"
#include "funcdata.h"

// callbackasm1 is the second part of the callback trampoline.
// On entry:
//   - CX contains the callback index (set by callbackasm)
//   - 0(SP) contains the return address to C caller
//   - 4(SP), 8(SP), ... contain C arguments (cdecl convention)
//
// i386 cdecl calling convention:
// - All arguments passed on stack
// - Return value in EAX (and EDX for 64-bit)
// - Caller cleans the stack
// - Callee must preserve: EBX, ESI, EDI, EBP
TEXT callbackasm1(SB), NOSPLIT|NOFRAME, $0
	NO_LOCAL_POINTERS

	// Save the return address
	MOVL 0(SP), AX

	// Allocate stack frame (must be done carefully to preserve args access)
	// Layout:
	//   0-15: saved callee-saved registers (BX, SI, DI, BP)
	//   16-19: saved callback index
	//   20-23: saved return address
	//   24-35: callbackArgs struct (12 bytes)
	//   36-291: copy of C arguments (256 bytes for 64 args, matching callbackMaxFrame)
	// Total: 292 bytes, round up to 304 for alignment
	SUBL $304, SP

	// Save callee-saved registers
	MOVL BX, 0(SP)
	MOVL SI, 4(SP)
	MOVL DI, 8(SP)
	MOVL BP, 12(SP)

	// Save callback index and return address
	MOVL CX, 16(SP)
	MOVL AX, 20(SP)

	// Copy C arguments from original stack location to our frame
	// Original args start at 304+4(SP) = 308(SP) (past our frame + original return addr)
	// Copy to our frame at 36(SP)
	// Copy 64 arguments (256 bytes, matching callbackMaxFrame = 64 * ptrSize)
	MOVL 308(SP), AX
	MOVL AX, 36(SP)
	MOVL 312(SP), AX
	MOVL AX, 40(SP)
	MOVL 316(SP), AX
	MOVL AX, 44(SP)
	MOVL 320(SP), AX
	MOVL AX, 48(SP)
	MOVL 324(SP), AX
	MOVL AX, 52(SP)
	MOVL 328(SP), AX
	MOVL AX, 56(SP)
	MOVL 332(SP), AX
	MOVL AX, 60(SP)
	MOVL 336(SP), AX
	MOVL AX, 64(SP)
	MOVL 340(SP), AX
	MOVL AX, 68(SP)
	MOVL 344(SP), AX
	MOVL AX, 72(SP)
	MOVL 348(SP), AX
	MOVL AX, 76(SP)
	MOVL 352(SP), AX
	MOVL AX, 80(SP)
	MOVL 356(SP), AX
	MOVL AX, 84(SP)
	MOVL 360(SP), AX
	MOVL AX, 88(SP)
	MOVL 364(SP), AX
	MOVL AX, 92(SP)
	MOVL 368(SP), AX
	MOVL AX, 96(SP)
	MOVL 372(SP), AX
	MOVL AX, 100(SP)
	MOVL 376(SP), AX
	MOVL AX, 104(SP)
	MOVL 380(SP), AX
	MOVL AX, 108(SP)
	MOVL 384(SP), AX
	MOVL AX, 112(SP)
	MOVL 388(SP), AX
	MOVL AX, 116(SP)
	MOVL 392(SP), AX
	MOVL AX, 120(SP)
	MOVL 396(SP), AX
	MOVL AX, 124(SP)
	MOVL 400(SP), AX
	MOVL AX, 128(SP)
	MOVL 404(SP), AX
	MOVL AX, 132(SP)
	MOVL 408(SP), AX
	MOVL AX, 136(SP)
	MOVL 412(SP), AX
	MOVL AX, 140(SP)
	MOVL 416(SP), AX
	MOVL AX, 144(SP)
	MOVL 420(SP), AX
	MOVL AX, 148(SP)
	MOVL 424(SP), AX
	MOVL AX, 152(SP)
	MOVL 428(SP), AX
	MOVL AX, 156(SP)
	MOVL 432(SP), AX
	MOVL AX, 160(SP)
	MOVL 436(SP), AX
	MOVL AX, 164(SP)
	MOVL 440(SP), AX
	MOVL AX, 168(SP)
	MOVL 444(SP), AX
	MOVL AX, 172(SP)
	MOVL 448(SP), AX
	MOVL AX, 176(SP)
	MOVL 452(SP), AX
	MOVL AX, 180(SP)
	MOVL 456(SP), AX
	MOVL AX, 184(SP)
	MOVL 460(SP), AX
	MOVL AX, 188(SP)
	MOVL 464(SP), AX
	MOVL AX, 192(SP)
	MOVL 468(SP), AX
	MOVL AX, 196(SP)
	MOVL 472(SP), AX
	MOVL AX, 200(SP)
	MOVL 476(SP), AX
	MOVL AX, 204(SP)
	MOVL 480(SP), AX
	MOVL AX, 208(SP)
	MOVL 484(SP), AX
	MOVL AX, 212(SP)
	MOVL 488(SP), AX
	MOVL AX, 216(SP)
	MOVL 492(SP), AX
	MOVL AX, 220(SP)
	MOVL 496(SP), AX
	MOVL AX, 224(SP)
	MOVL 500(SP), AX
	MOVL AX, 228(SP)
	MOVL 504(SP), AX
	MOVL AX, 232(SP)
	MOVL 508(SP), AX
	MOVL AX, 236(SP)
	MOVL 512(SP), AX
	MOVL AX, 240(SP)
	MOVL 516(SP), AX
	MOVL AX, 244(SP)
	MOVL 520(SP), AX
	MOVL AX, 248(SP)
	MOVL 524(SP), AX
	MOVL AX, 252(SP)
	MOVL 528(SP), AX
	MOVL AX, 256(SP)
	MOVL 532(SP), AX
	MOVL AX, 260(SP)
	MOVL 536(SP), AX
	MOVL AX, 264(SP)
	MOVL 540(SP), AX
	MOVL AX, 268(SP)
	MOVL 544(SP), AX
	MOVL AX, 272(SP)
	MOVL 548(SP), AX
	MOVL AX, 276(SP)
	MOVL 552(SP), AX
	MOVL AX, 280(SP)
	MOVL 556(SP), AX
	MOVL AX, 284(SP)
	MOVL 560(SP), AX
	MOVL AX, 288(SP)

	// Set up callbackArgs struct at 24(SP)
	// struct callbackArgs {
	//     index  uintptr  // offset 0
	//     args   *byte    // offset 4
	//     result uintptr  // offset 8
	// }
	MOVL 16(SP), AX // callback index
	MOVL AX, 24(SP) // callbackArgs.index
	LEAL 36(SP), AX // pointer to copied arguments
	MOVL AX, 28(SP) // callbackArgs.args
	MOVL $0, 32(SP) // callbackArgs.result = 0

	// Call crosscall2(fn, frame, 0, ctxt)
	// crosscall2 expects arguments on stack:
	//   0(SP) = fn
	//   4(SP) = frame (pointer to callbackArgs)
	//   8(SP) = ignored (was n)
	//   12(SP) = ctxt
	SUBL $16, SP

	MOVL ·callbackWrap_call(SB), AX
	MOVL (AX), AX                   // fn = *callbackWrap_call
	MOVL AX, 0(SP)                  // fn
	LEAL (24+16)(SP), AX            // &callbackArgs (adjusted for SUB $16)
	MOVL AX, 4(SP)                  // frame
	MOVL $0, 8(SP)                  // 0
	MOVL $0, 12(SP)                 // ctxt

	CALL crosscall2(SB)

	ADDL $16, SP

	// Get result from callbackArgs.result
	MOVL 32(SP), AX

	// Restore callee-saved registers
	MOVL 0(SP), BX
	MOVL 4(SP), SI
	MOVL 8(SP), DI
	MOVL 12(SP), BP

	// Restore return address and clean up
	MOVL 20(SP), CX // get return address
	ADDL $304, SP   // remove our frame
	MOVL CX, 0(SP)  // put return address back

	RET
