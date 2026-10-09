import { createLoan } from './actions';

export default function NewLoanPage() {
  async function remind() {
    'use server';
  }
  return (
    <>
      <form action={createLoan}>
        <input name="bookId" />
      </form>
      <form action={remind}>
        <button>Remind me</button>
      </form>
    </>
  );
}
