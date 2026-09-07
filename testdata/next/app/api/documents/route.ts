import { NextResponse } from "next/server"

export async function GET(request: Request) {
  return NextResponse.json({ documents: [] })
}

export async function POST(request: Request) {
  const body = await request.json()
  return NextResponse.json(body, { status: 201 })
}

export async function DELETE(request: Request) {
  return new NextResponse(null, { status: 204 })
}
