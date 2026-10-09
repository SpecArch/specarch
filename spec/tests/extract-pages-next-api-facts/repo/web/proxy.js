import { NextResponse } from 'next/server';

export const config = { matcher: ['/((?!api|_next).*)'] };

export function proxy() {
  return NextResponse.next();
}
