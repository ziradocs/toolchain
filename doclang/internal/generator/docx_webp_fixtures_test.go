// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

// Imágenes WebP de prueba, hechas con Pillow. El perfil de color es el sRGB
// que genera Pillow (ImageCms.createProfile("sRGB")), de 588 bytes.

// webpLossy24x16 es un WebP con pérdida y sin alfa, de 24×16, verde.
const webpLossy24x16 = "UklGRkQAAABXRUJQVlA4IDgAAADwAgCdASoYABAAPm0skUWkIqGYBABABsSgCdLoAAgbAAD+7U3f+A3Xxa5f/0Yf/hh/+GH9" +
	"K6EoAA=="

// webpLossyICC24x16 es webpLossy24x16 con perfil de color (chunk ICCP).
const webpLossyICC24x16 = "UklGRqoCAABXRUJQVlA4WAoAAAAgAAAAFwAADwAASUNDUEwCAAAAAAJMbGNtcwRAAABtbnRyUkdCIFhZWiAH6gALAAoACQAh" +
	"ACxhY3NwQVBQTAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA9tYAAQAAAADTLWxjbXMAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" +
	"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAtkZXNjAAABCAAAADZjcHJ0AAABQAAAAEx3dHB0AAABjAAAABRjaGFkAAABoAAA" +
	"ACxyWFlaAAABzAAAABRiWFlaAAAB4AAAABRnWFlaAAAB9AAAABRyVFJDAAACCAAAACBnVFJDAAACCAAAACBiVFJDAAACCAAA" +
	"ACBjaHJtAAACKAAAACRtbHVjAAAAAAAAAAEAAAAMZW5VUwAAABoAAAAcAHMAUgBHAEIAIABiAHUAaQBsAHQALQBpAG4AAG1s" +
	"dWMAAAAAAAAAAQAAAAxlblVTAAAAMAAAABwATgBvACAAYwBvAHAAeQByAGkAZwBoAHQALAAgAHUAcwBlACAAZgByAGUAZQBs" +
	"AHlYWVogAAAAAAAA9tYAAQAAAADTLXNmMzIAAAAAAAEMQgAABd7///MlAAAHkwAA/ZD///uh///9ogAAA9wAAMBuWFlaIAAA" +
	"AAAAAG+gAAA49QAAA5BYWVogAAAAAAAAJJ8AAA+EAAC2w1hZWiAAAAAAAABilwAAt4cAABjZcGFyYQAAAAAAAwAAAAJmZgAA" +
	"8qcAAA1ZAAAT0AAACltjaHJtAAAAAAADAAAAAKPXAABUewAATM0AAJmaAAAmZgAAD1xWUDggOAAAAPACAJ0BKhgAEAA+bSyR" +
	"RaQioZgEAEAGxKAJ0ugACBsAAP7tTd/4DdfFrl//Rh/+GH/4Yf0roSgA"

// webpLosslessICC8x8 es un WebP sin pérdida de 8×8 con perfil de color.
const webpLosslessICC8x8 = "UklGRoQCAABXRUJQVlA4WAoAAAAgAAAABwAABwAASUNDUEwCAAAAAAJMbGNtcwRAAABtbnRyUkdCIFhZWiAH6gALAAoACQAh" +
	"ACxhY3NwQVBQTAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA9tYAAQAAAADTLWxjbXMAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" +
	"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAtkZXNjAAABCAAAADZjcHJ0AAABQAAAAEx3dHB0AAABjAAAABRjaGFkAAABoAAA" +
	"ACxyWFlaAAABzAAAABRiWFlaAAAB4AAAABRnWFlaAAAB9AAAABRyVFJDAAACCAAAACBnVFJDAAACCAAAACBiVFJDAAACCAAA" +
	"ACBjaHJtAAACKAAAACRtbHVjAAAAAAAAAAEAAAAMZW5VUwAAABoAAAAcAHMAUgBHAEIAIABiAHUAaQBsAHQALQBpAG4AAG1s" +
	"dWMAAAAAAAAAAQAAAAxlblVTAAAAMAAAABwATgBvACAAYwBvAHAAeQByAGkAZwBoAHQALAAgAHUAcwBlACAAZgByAGUAZQBs" +
	"AHlYWVogAAAAAAAA9tYAAQAAAADTLXNmMzIAAAAAAAEMQgAABd7///MlAAAHkwAA/ZD///uh///9ogAAA9wAAMBuWFlaIAAA" +
	"AAAAAG+gAAA49QAAA5BYWVogAAAAAAAAJJ8AAA+EAAC2w1hZWiAAAAAAAABilwAAt4cAABjZcGFyYQAAAAAAAwAAAAJmZgAA" +
	"8qcAAA1ZAAAT0AAACltjaHJtAAAAAAADAAAAAKPXAABUewAATM0AAJmaAAAmZgAAD1xWUDhMEQAAAC8HwAEAB1CPItej/4GI" +
	"6H8AAA=="

// WebP animados de prueba, de 16×12 y dos cuadros. El segundo es azul opaco:
// si una imagen convertida sale azul, se tomó el cuadro equivocado. Hechos
// con Pillow 12.1.1 y libwebp 1.5.0 (los bytes dependen de la versión de
// libwebp); este programa los imprime en base64, en el orden de abajo:
//
//	import base64, io
//	from PIL import Image
//
//	blue = Image.new("RGBA", (16, 12), (0, 0, 255, 255))
//
//	def anim(f1, opts, f2=blue):
//	    b = io.BytesIO()
//	    f1.save(b, "WEBP", save_all=True, append_images=[f2], duration=100, **opts)
//	    return base64.b64encode(b.getvalue()).decode()
//
//	square = Image.new("RGBA", (16, 12), (0, 0, 0, 0))
//	square.paste((255, 0, 0, 255), (6, 4, 12, 10))
//	print(anim(Image.new("RGBA", (16, 12), (255, 0, 0, 255)), {"quality": 80}))
//	print(anim(Image.new("RGBA", (16, 12), (0, 255, 0, 255)), {"lossless": True}))
//	print(anim(square, {"quality": 80}))
//	print(anim(Image.new("RGBA", (16, 12), (255, 0, 0, 128)), {"quality": 80}))
//	print(anim(Image.new("RGBA", (16, 12), (255, 0, 0, 255)), {"quality": 80},
//	           Image.new("RGBA", (16, 12), (0, 0, 255, 128))))
const (
	// webpAnimLossy16x12: el primer cuadro es rojo, con pérdida (VP8), del
	// tamaño del lienzo.
	webpAnimLossy16x12 = "UklGRsQAAABXRUJQVlA4WAoAAAACAAAADwAACwAAQU5JTQYAAAAAAAAAAABBTk1GSgAAAAAAAAAAAA8AAAsAAGQAAAJWUDggMgAAADABAJ0BKhAADAABQCYloAADcAD+8ut///mwP/bz/wR6Af//0uD//pcH//S4P/SkAAAAQU5NRkYAAAAAAAAAAAAPAAALAABkAAAAVlA4IC4AAAA0AQCdASoQAAwAAAAmJaAAA3AA/vtV4///S4P/+lwf/9Lg/9Lg//rV5Vesq6AA"
	// webpAnimLossless16x12: el primer cuadro es verde, sin pérdida (VP8L),
	// del tamaño del lienzo.
	webpAnimLossless16x12 = "UklGRoQAAABXRUJQVlA4WAoAAAACAAAADwAACwAAQU5JTQYAAAAAAAAAAABBTk1GKAAAAAAAAAAAAA8AAAsAAGQAAAJWUDhMDwAAAC8PwAIAB9D/iP4HIqL/AQBBTk1GKAAAAAAAAAAAAA8AAAsAAGQAAABWUDhMDwAAAC8PwAIABxDR//4HIqL/AQA="
	// webpAnimOffset16x12: el primer cuadro es un cuadrado rojo opaco en
	// (6,4)-(12,10) sobre transparente. libwebp recorta el borde
	// transparente, así que su ANMF es de 6×6 en (6,4), con VP8 y sin ALPH.
	webpAnimOffset16x12 = "UklGRsQAAABXRUJQVlA4WAoAAAACAAAADwAACwAAQU5JTQYAAAAAAAAAAABBTk1GSgAAAAMAAAIAAAUAAAUAAGQAAAJWUDggMgAAADABAJ0BKgYABgABQCYloAADcAD+8ut///mwP/bz/wR6Af//0uD//pcH//S4P/SkAAAAQU5NRkYAAAAAAAAAAAAPAAALAABkAAAAVlA4IC4AAAA0AQCdASoQAAwAAAAmJaAAA3AA/vtV4///S4P/+lwf/9Lg/9Lg//rV5Vesq6AA"
	// webpAnimAlpha16x12: el primer cuadro es rojo con alfa 128, con pérdida
	// (ALPH y VP8 dentro del ANMF).
	webpAnimAlpha16x12 = "UklGRtYAAABXRUJQVlA4WAoAAAASAAAADwAACwAAQU5JTQYAAAAAAAAAAABBTk1GXAAAAAAAAAAAAA8AAAsAAGQAAAJBTFBICgAAAAEHUMCICERE/wNWUDggMgAAADABAJ0BKhAADAABQCYloAADcAD+8ut///mwP/bz/wR6Af//0uD//pcH//S4P/SkAAAAQU5NRkYAAAAAAAAAAAAPAAALAABkAAAAVlA4IC4AAAA0AQCdASoQAAwAAAAmJaAAA3AA/vtV4///S4P/+lwf/9Lg/9Lg//rV5Vesq6AA"
	// webpAnimCanvasAlpha16x12: el lienzo lleva la marca de alfa (por el
	// segundo cuadro, translúcido), pero el primero es rojo opaco, con VP8 y
	// sin ALPH.
	webpAnimCanvasAlpha16x12 = "UklGRtYAAABXRUJQVlA4WAoAAAASAAAADwAACwAAQU5JTQYAAAAAAAAAAABBTk1GSgAAAAAAAAAAAA8AAAsAAGQAAAJWUDggMgAAADABAJ0BKhAADAABQCYloAADcAD+8ut///mwP/bz/wR6Af//0uD//pcH//S4P/SkAAAAQU5NRlgAAAAAAAAAAAAPAAALAABkAAACQUxQSAoAAAABB1DAiAhERP8DVlA4IC4AAAAwAQCdASoQAAwAAUAmJaAAA3AA/vtV4///S4P/+lwf/9Lg/9Lg//rV5Vesq6AA"
)
