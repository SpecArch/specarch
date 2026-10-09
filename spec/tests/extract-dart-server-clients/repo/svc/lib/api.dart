import 'package:dio/dio.dart';
import 'package:retrofit/retrofit.dart';
import 'package:http/http.dart' as http;

part 'api.g.dart';

@RestApi(baseUrl: 'https://archive.example.org/v2')
abstract class ArchiveApi {
  @GET('/notes/{id}')
  Future<String> getNote(@Path('id') String id);

  @POST('/notes')
  Future<void> store(@Body() Map<String, dynamic> note);
}

Future<void> ping(Dio dio, String id) async {
  await dio.get('https://status.example.org/ping');
  await dio.post('/local/notes');
  await http.get(Uri.parse('https://weather.example.org/today/$id'));
}
