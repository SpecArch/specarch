import 'package:json_annotation/json_annotation.dart';

part 'note.g.dart';

enum Mood {
  @JsonValue('calm')
  calm,
  @JsonValue('in_a_hurry')
  hurried,
  @JsonValue('Very Happy')
  happy,
}

@JsonSerializable(fieldRename: FieldRename.snake)
class Note {
  Note({required this.id, required this.title, this.body, required this.pages, required this.createdAt, required this.mood});

  final String id;
  final String title;
  final String? body;
  final int pages;
  final DateTime createdAt;
  final Mood mood;
  final List<String> tags = const [];
  final Map<String, String> extra = const {};
  @JsonKey(includeFromJson: false, includeToJson: false)
  final bool dirty = false;
  static const kind = 'note';
}

@JsonSerializable()
class Author {
  Author(this.name, this.email, this.rating, this.score);

  final String name;
  @JsonKey(name: 'mail_address')
  final String email;
  @JsonKey(name: 'stars')
  final num rating;
  final double score;
  final Note? favourite;
  final Uri? site;
  final Contact contact;
}
