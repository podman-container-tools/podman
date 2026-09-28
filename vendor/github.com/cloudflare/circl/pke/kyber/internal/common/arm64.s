//go:build arm64 && !purego

#include "go_asm.h"
#include "textflag.h"

// func polyAddARM64(p, a, b *Poly)
TEXT ·polyAddARM64(SB), NOSPLIT|NOFRAME, $0-24
    MOVD    p+0(FP), R0
    MOVD    a+8(FP), R1
    MOVD    b+16(FP), R2

    MOVW    $(const_N / 32), R3

loop:
    VLD1.P  (64)(R1), [V0.H8, V1.H8, V2.H8, V3.H8]
    VLD1.P  (64)(R2), [V4.H8, V5.H8, V6.H8, V7.H8]

    VADD    V4.H8, V0.H8, V0.H8
    VADD    V5.H8, V1.H8, V1.H8
    VADD    V6.H8, V2.H8, V2.H8
    VADD    V7.H8, V3.H8, V3.H8

    VST1.P  [V0.H8, V1.H8, V2.H8, V3.H8], (64)(R0)

    SUBS    $1, R3, R3
    BGT     loop

    RET


// func polySubARM64(p, a, b *Poly)
TEXT ·polySubARM64(SB), NOSPLIT|NOFRAME, $0-24
    MOVD    p+0(FP), R0
    MOVD    a+8(FP), R1
    MOVD    b+16(FP), R2

    MOVW    $(const_N / 32), R3

loop:
    VLD1.P  (64)(R1), [V0.H8, V1.H8, V2.H8, V3.H8]
    VLD1.P  (64)(R2), [V4.H8, V5.H8, V6.H8, V7.H8]

    VSUB    V4.H8, V0.H8, V0.H8
    VSUB    V5.H8, V1.H8, V1.H8
    VSUB    V6.H8, V2.H8, V2.H8
    VSUB    V7.H8, V3.H8, V3.H8

    VST1.P  [V0.H8, V1.H8, V2.H8, V3.H8], (64)(R0)

    SUBS    $1, R3, R3
    BGT     loop

    RET
