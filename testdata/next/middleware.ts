import { NextResponse } from "next/server"

export function middleware(request: Request) {
  const session = request.headers.get("cookie")
  if (!session) return NextResponse.redirect(new URL("/login", request.url))
  return NextResponse.next()
}

export const config = { matcher: ["/dashboard/:path*"] }
