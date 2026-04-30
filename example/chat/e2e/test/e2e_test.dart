import 'package:chat_api/chat_api.dart';
import 'package:test/test.dart';

void main() {
  const baseUrl = 'http://127.0.0.1:8280';
  late ApiClient client;

  setUp(() {
    client = ApiClient(
      baseUrl: baseUrl,
      customerAuthorization: () async => 'Customer customer_123',
      merchantAuthorization: () async => 'Merchant merchant_456 store_main',
    );
  });

  group('chat API SDK e2e', () {
    test('customer and merchant exchange messages', () async {
      final started = expectSuccess(
        await client.customer.startConversation(
          body: const StartConversationRequest(
            storeId: 'store_main',
            message: 'Is this lamp still available?',
          ),
        ),
      );

      expect(started.id, startsWith('conv_'));
      expect(started.storeId, 'store_main');
      expect(started.customerId, 'customer_123');
      expect(started.messages, hasLength(1));
      expect(started.messages.single.senderKind, 'customer');
      expect(started.messages.single.senderId, 'customer_123');
      expect(started.messages.single.text, 'Is this lamp still available?');

      final afterCustomerMessage = expectSuccess(
        await client.customer.addMessage(
          id: started.id,
          body: const AddMessageRequest(
            message: 'I can pick it up this afternoon.',
          ),
        ),
      );

      expect(afterCustomerMessage.messages, hasLength(2));
      expect(afterCustomerMessage.messages.last.senderKind, 'customer');
      expect(
        afterCustomerMessage.messages.last.text,
        'I can pick it up this afternoon.',
      );

      final merchantRead = expectSuccess(
        await client.merchant.getConversation(id: started.id),
      );

      expect(merchantRead.messages, hasLength(2));

      final afterMerchantReply = expectSuccess(
        await client.merchant.replyToConversation(
          id: started.id,
          body: const ReplyToConversationRequest(
            message: 'Yes, it is available today.',
          ),
        ),
      );

      expect(afterMerchantReply.messages, hasLength(3));
      expect(afterMerchantReply.messages.last.senderKind, 'merchant');
      expect(afterMerchantReply.messages.last.senderId, 'merchant_456');
      expect(
          afterMerchantReply.messages.last.text, 'Yes, it is available today.');

      final customerRead = expectSuccess(
        await client.customer.getConversation(id: started.id),
      );

      expect(customerRead.messages, hasLength(3));
      expect(customerRead.messages.last.senderKind, 'merchant');
    });

    test('customer group rejects a merchant authorization header', () async {
      final invalidCustomerClient = ApiClient(
        baseUrl: baseUrl,
        customerAuthorization: () async => 'Merchant merchant_456 store_main',
        merchantAuthorization: () async => 'Merchant merchant_456 store_main',
      );

      final error = expectApiException(
        await invalidCustomerClient.customer.startConversation(
          body: const StartConversationRequest(
            storeId: 'store_main',
            message: 'This should not be accepted.',
          ),
        ),
        statusCode: 400,
      );

      expect(error.data.code, 'invalid_header_parameter');
    });

    test('merchant cannot read another store conversation', () async {
      final started = expectSuccess(
        await client.customer.startConversation(
          body: const StartConversationRequest(
            storeId: 'store_main',
            message: 'Can another store see this?',
          ),
        ),
      );
      final otherStoreClient = ApiClient(
        baseUrl: baseUrl,
        customerAuthorization: () async => 'Customer customer_123',
        merchantAuthorization: () async => 'Merchant merchant_999 store_other',
      );

      final error = expectApiException(
        await otherStoreClient.merchant.getConversation(id: started.id),
        statusCode: 403,
      );

      expect(error.data.code, 'wrong_store');
    });

    test('customer cannot read another customer conversation', () async {
      final started = expectSuccess(
        await client.customer.startConversation(
          body: const StartConversationRequest(
            storeId: 'store_main',
            message: 'Private customer thread.',
          ),
        ),
      );
      final otherCustomerClient = ApiClient(
        baseUrl: baseUrl,
        customerAuthorization: () async => 'Customer customer_999',
        merchantAuthorization: () async => 'Merchant merchant_456 store_main',
      );

      final error = expectApiException(
        await otherCustomerClient.customer.getConversation(id: started.id),
        statusCode: 404,
      );

      expect(error.data.code, 'conversation_not_found');
    });
  });
}

T expectSuccess<T, E>(Result<T, E> result) {
  if (result is Success<T, E>) {
    final data = result.value.data;
    expect(data, isNotNull);
    return data as T;
  }
  fail('Expected Success<$T, $E>, got ${_describeResult(result)}');
}

ApiException<T, E> expectApiException<T, E>(
  Result<T, E> result, {
  required int statusCode,
}) {
  if (result is ApiException<T, E>) {
    expect(result.statusCode, statusCode);
    return result;
  }
  fail('Expected ApiException<$T, $E>, got ${_describeResult(result)}');
}

String _describeResult<T, E>(Result<T, E> result) {
  if (result is ApiException<T, E>) {
    return 'ApiException(statusCode: ${result.statusCode}, '
        'data: ${_describeErrorData(result.data)}, rawBody: ${result.rawBody})';
  }
  return result.toString();
}

String _describeErrorData(Object? data) {
  if (data is DefaultError) {
    return 'DefaultError(code: ${data.code}, message: ${data.message}, '
        'details: ${data.details})';
  }
  return data.toString();
}
