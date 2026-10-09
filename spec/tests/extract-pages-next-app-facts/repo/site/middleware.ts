import { NextResponse, type NextRequest } from 'next/server';
import { requirePermission } from './lib/auth';

export const config = { matcher: ['/loans/:path*'] };

export async function middleware(request: NextRequest) {
  await requirePermission('loans.read');
  return NextResponse.next();
}
