import Link from "next/link";

export default function DeskHome() {
  return (
    <main>
      <h1>Lending desk</h1>
      <p>
        <Link href="/loans/new">Lend a book</Link>
      </p>
    </main>
  );
}
