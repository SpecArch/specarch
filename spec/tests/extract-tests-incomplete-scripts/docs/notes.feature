Feature: Keeping notes

  Background:
    Given a signed-in writer

  @positive
  Scenario: Keep a note
    When the writer sends POST /notes with a text
    Then it answers 201 with the note

  Scenario: Keep an empty note
    When the writer sends POST /notes with no text

  Scenario Outline: Read a note
    Given a note <id>
    When the writer reads it
    Then it answers <status>

    Examples:
      | id | status |
      | n1 | 200    |
      | n9 | 404    |
