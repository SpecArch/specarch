import { ScreenForm } from "@acme/screens";

export default function LendBookPage() {
  return <ScreenForm operation="lendBook" fields={["cardNumber", "barcode"]} />;
}
