//go:build arm64 && !purego

#include "go_asm.h"
#include "textflag.h"

// func polyAddARM64(p, a, b *Poly)
TEXT ·polyAddARM64(SB), NOSPLIT|NOFRAME, $0-24
    MOVD    p+0(FP), R0
    MOVD    a+8(FP), R1
    MOVD    b+16(FP), R2

    MOVW    $(const_N / 16), R3 // loop iterations (for each iteration we emit 16 elements)

loop:
    VLD1.P  (64)(R1), [V0.S4, V1.S4, V2.S4, V3.S4]
    VLD1.P  (64)(R2), [V4.S4, V5.S4, V6.S4, V7.S4]

    VADD    V4.S4, V0.S4, V0.S4
    VADD    V5.S4, V1.S4, V1.S4
    VADD    V6.S4, V2.S4, V2.S4
    VADD    V7.S4, V3.S4, V3.S4

    VST1.P  [V0.S4, V1.S4, V2.S4, V3.S4], (64)(R0)

    SUBS    $1, R3, R3
    BGT     loop

    RET


// func polyPackLe16ARM64(p *Poly, buf *byte)
TEXT ·polyPackLe16ARM64(SB), NOSPLIT|NOFRAME, $0-16
    MOVD    p+0(FP), R0
    MOVD    buf+8(FP), R1

    MOVW    $(const_PolyLe16Size / 16), R3 // loop iterations (for each iteration we emit 16 elements)
    VMOVQ   $0x1c0c180814041000, $0x3c2c382834243020, V15 // value explained at VTBL call

    // on the first iteration we have:
    // V0 = (p[0], p[4], p[8], p[12])       V1 = (p[1], p[5], p[9], p[13])
    // V2 = (p[2], p[6], p[10], p[14])      V3 = (p[3], p[7], p[11], p[15])
    // V4 = (p[16], p[20], p[24], p[28])    V5 = (p[17], p[21], p[25], p[29])
    // V6 = (p[18], p[22], p[26], p[30])    V7 = (p[19], p[23], p[27], p[31])
loop:
    VLD4.P  (64)(R0), [V0.S4, V1.S4, V2.S4, V3.S4]
    VLD4.P  (64)(R0), [V4.S4, V5.S4, V6.S4, V7.S4]

    VSHL    $4, V1.S4, V1.S4
    VSHL    $4, V3.S4, V3.S4
    VSHL    $4, V5.S4, V5.S4
    VSHL    $4, V7.S4, V7.S4

    // tmp = p[even] | (p[odd] << 4)
    VORR    V1.B16, V0.B16, V10.B16
    VORR    V3.B16, V2.B16, V11.B16
    VORR    V5.B16, V4.B16, V12.B16
    VORR    V7.B16, V6.B16, V13.B16

    // so now we need to pick elements based on order:
    // first from V10; first from V11; second from V10; second from V11;
    // ...
    // first from V12; first from V13; second from V12; second from V13;
    // V15 contains the indices which correspond to the pick order above
    VTBL    V15.B16, [V10.B16, V11.B16, V12.B16, V13.B16], V16.B16

    VST1.P  [V16.B16], (16)(R1)

    SUBS    $1, R3, R3
    BGT     loop

    RET

// func polyMulBy2toDARM64(p, q *Poly)
TEXT ·polyMulBy2toDARM64(SB), NOSPLIT|NOFRAME, $0-16
    MOVD    p+0(FP), R0
    MOVD    q+8(FP), R1

    MOVW    $(const_N / 16), R2

loop:
    VLD1.P  (64)(R1), [V0.S4, V1.S4, V2.S4, V3.S4]

    VSHL    $(const_D), V0.S4, V0.S4
    VSHL    $(const_D), V1.S4, V1.S4
    VSHL    $(const_D), V2.S4, V2.S4
    VSHL    $(const_D), V3.S4, V3.S4

    VST1.P  [V0.S4, V1.S4, V2.S4, V3.S4], (64)(R0)

    SUBS    $1, R2, R2
    BGT     loop

    RET

// func polySubARM64(p, a, b *Poly)
TEXT ·polySubARM64(SB), NOSPLIT|NOFRAME, $0-24
    MOVD    p+0(FP), R0
    MOVD    a+8(FP), R1
    MOVD    b+16(FP), R2

    MOVW    $(const_N / 16), R3
    MOVW    $(const_Q << 1), R4

    VDUP    R4, V8.S4

    // p = a + (2q - b)
loop:
    VLD1.P  (64)(R1), [V0.S4, V1.S4, V2.S4, V3.S4]
    VLD1.P  (64)(R2), [V4.S4, V5.S4, V6.S4, V7.S4]

    VSUB    V4.S4, V8.S4, V4.S4
    VSUB    V5.S4, V8.S4, V5.S4
    VSUB    V6.S4, V8.S4, V6.S4
    VSUB    V7.S4, V8.S4, V7.S4

    VADD    V4.S4, V0.S4, V0.S4
    VADD    V5.S4, V1.S4, V1.S4
    VADD    V6.S4, V2.S4, V2.S4
    VADD    V7.S4, V3.S4, V3.S4

    VST1.P  [V0.S4, V1.S4, V2.S4, V3.S4], (64)(R0)

    SUBS    $1, R3, R3
    BGT     loop

    RET
