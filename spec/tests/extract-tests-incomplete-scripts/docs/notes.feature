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

  Scenario: Read a note
    When the writer reads a note by its title
    Then it answers 200

  Scenario: Read a note
    When the writer reads a note kept by another writer
    Then it answers 403

  Scenario Outline: Share a note
    When the writer shares it with <reader> at <time>
    Then it answers <status>

    Examples:
      | reader | status |
      | ann    | 201    |
      |        |        |
